package main

import (
 "context"
 "encoding/json"
 "net/http"
 "net/url"
 "sync"
 "time"

 "github.com/coder/websocket"
)

// Sideband: provider owns media; backend retains authorization and metering.
// No end/duration parameter from the browser is ever trusted for billing.
func (a *App) voiceObserveLive(callID, sessionID string, maxMinutes int) {
 if maxMinutes<1||maxMinutes>15{return}
 ctx,cancel:=context.WithTimeout(context.Background(),time.Duration(maxMinutes+4)*time.Minute)
 defer cancel()
 headers:=http.Header{}
 headers.Set("Authorization","Bearer "+voiceKey("OPENAI_API_KEY"))
 addr:="wss://api.openai.com/v1/live/sessions/"+url.PathEscape(sessionID)+"/attach"
 conn,_,err:=websocket.Dial(ctx,addr,&websocket.DialOptions{HTTPHeader:headers})
 if err!=nil{return} // Stale recovery retains the paid reservation.
 defer conn.Close(websocket.StatusNormalClosure,"")
 var lock sync.Mutex
 var started time.Time
 var limitTimer *time.Timer
 finished:=false
 // Includes up to 3 minutes of ringing/setup; forcibly end a stuck session.
 terminate:=func(){
  lock.Lock();if finished{lock.Unlock();return};finished=true;lock.Unlock()
  ctx2,stop:=context.WithTimeout(context.Background(),8*time.Second)
  _,_,_=voicePostOpenAI(ctx2,"live/sessions/"+url.PathEscape(sessionID)+"/hangup",nil)
  stop()
  _=a.voiceSettle(callID,"completed",maxMinutes*60,"time_limit")
  _=conn.Close(websocket.StatusNormalClosure,"time limit")
 }
 hardTimer:=time.AfterFunc(time.Duration(maxMinutes+3)*time.Minute,terminate)
 defer hardTimer.Stop()
 defer func(){lock.Lock();finished=true;if limitTimer!=nil{limitTimer.Stop()};lock.Unlock()}()
 for {
  _,payload,err:=conn.Read(ctx)
  if err!=nil{return}
  var ev struct {Type string `json:"type"`;Reason string `json:"reason"`;DurationSeconds int `json:"duration_seconds"`}
  if json.Unmarshal(payload,&ev)!=nil{continue}
  switch ev.Type{
  case "session.started","transport.answered":
   lock.Lock()
   if started.IsZero(){started=time.Now();limitTimer=time.AfterFunc(time.Duration(maxMinutes)*time.Minute,terminate)}
   lock.Unlock()
  case "session.closed":
   lock.Lock();if finished{lock.Unlock();return};finished=true;begin:=started;lock.Unlock()
   seconds:=0
   if !begin.IsZero(){seconds=int(time.Since(begin).Seconds());if seconds<1{seconds=1}}
   if ev.DurationSeconds>0{seconds=ev.DurationSeconds}
   if seconds>maxMinutes*60{seconds=maxMinutes*60}
   _=a.voiceSettle(callID,"completed",seconds,ev.Reason)
   return
  case "transport.failed":
   lock.Lock();if finished{lock.Unlock();return};finished=true;lock.Unlock()
   _=a.voiceSettle(callID,"failed",0,"transport_failed")
   return
  }
 }
}

// Recovery is intentionally conservative: after an unexpected crash, release
// reservations by charging no more than the customer-approved maximum.
func (a *App) voiceRecoverStale() {
 cutoff:=time.Now().UTC().Add(-25*time.Minute).Format(time.RFC3339Nano)
 rows,err:=a.db.Query(`SELECT c.id,ch.max_minutes FROM voice_calls_v2735 c JOIN voice_channels_v2735 ch ON ch.id=c.channel_id WHERE c.status IN ('reserved','active','accepting','uncertain') AND c.created_at<? LIMIT 100`,cutoff)
 if err!=nil{return}
 type stale struct{id string;minutes int};var items []stale
 for rows.Next(){var x stale;if rows.Scan(&x.id,&x.minutes)==nil{items=append(items,x)}}
 rows.Close()
 for _,x:=range items{_ = a.voiceSettle(x.id,"completed",x.minutes*60,"recovery_unverified_max_reservation")}
}
