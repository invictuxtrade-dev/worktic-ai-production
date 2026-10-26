import unittest, os, urllib.request, urllib.error, http.server, threading, socketserver
from staging_load import validate_url, main, trial
class SafeLoad(unittest.TestCase):
 def test_production_refused(self):
  for x in ('https://workticai.com','https://app.workticai.com','https://www.workticai.com'):
   with self.assertRaises(ValueError):validate_url(x)
 def test_staging_requires_https(self):
  with self.assertRaises(ValueError):validate_url('http://worktic-staging.onrender.com')
 def test_allow_staging(self):
  self.assertEqual(validate_url('https://worktic-staging.onrender.com/'),'https://worktic-staging.onrender.com')
 def test_reject_embedded_credentials(self):
  with self.assertRaises(ValueError):validate_url('https://token:secret@worktic-staging.onrender.com')
 def test_reject_query(self):
  with self.assertRaises(ValueError):validate_url('https://worktic-staging.onrender.com/?key=abc')
 def test_production_subdomain_without_explicit_qa_blocked(self):
  with self.assertRaises(ValueError):validate_url('https://foo.workticai.com')
 def test_no_nonhealth_paths(self):
  with self.assertRaises(ValueError):validate_url('https://worktic-staging.onrender.com/api/studio/video/generate')
 def test_forbid_staging_subdomain_on_production_base(self):
  for url in ('https://staging.workticai.com', 'https://qa.workticai.com', 'https://test.workticai.com','https://staging.worktic.com.co'):
   with self.assertRaises(ValueError):validate_url(url)
 def test_no_redirect_to_any_destination(self):
  class Redirect(http.server.BaseHTTPRequestHandler):
   calls=0
   def do_GET(self):
    type(self).calls+=1
    self.send_response(302);self.send_header('Location','https://workticai.com/healthz');self.end_headers()
   def log_message(self,*args):pass
  server=http.server.HTTPServer(('127.0.0.1',0),Redirect)
  worker=threading.Thread(target=server.serve_forever,daemon=True);worker.start()
  try:
   status,_=trial('http://127.0.0.1:'+str(server.server_port),'/healthz',2)
   self.assertEqual(status,302)
   self.assertEqual(Redirect.calls,1)
  finally:server.shutdown();server.server_close();worker.join(timeout=2)
 def test_requires_optin(self):
  with self.assertRaises(SystemExit):main(['--base','https://worktic-staging.onrender.com','--requests','1'])
if __name__=='__main__':unittest.main()
