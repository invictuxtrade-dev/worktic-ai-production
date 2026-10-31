package workticaccountmail

import (
	"bytes"
	"html/template"
	"strings"
)

const mailShell = `<!doctype html><html lang="es"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width"><title>Worktic AI</title></head><body style="margin:0;padding:30px 14px;background:#eef2fb;font-family:Arial,Helvetica,sans-serif;color:#15254b"><table role="presentation" style="width:100%;max-width:580px;margin:auto;border-spacing:0"><tr><td style="background:#071735;padding:22px 30px;border-radius:24px 24px 0 0"><img src="https://workticai.com/assets/logo-mark.png" alt="Worktic AI" width="46" height="46" style="vertical-align:middle;object-fit:contain;margin-right:10px"><span style="color:white;font-size:24px;font-weight:800;vertical-align:middle">Worktic <span style="color:#d23cff">AI</span></span></td></tr><tr><td style="background:white;padding:35px 30px;border-radius:0 0 24px 24px"><h1 style="color:#132249;font-size:25px;line-height:1.3">{{.Title}}</h1><p style="font-size:16px;line-height:1.6;color:#475477">{{.Body}}</p>{{if .Link}}<p style="margin:34px 0"><a href="{{.Link}}" style="display:inline-block;background:#6451e9;color:white;padding:16px 25px;border-radius:12px;text-decoration:none;font-weight:bold">{{.Button}}</a></p><p style="font-size:12px;overflow-wrap:anywhere;color:#66738f">{{.Link}}</p>{{end}}<p style="font-size:14px;line-height:1.6;color:#66738f">{{.Footer}}</p><hr style="border:0;border-top:1px solid #e8eaf6;margin:25px 0"><p style="color:#8290ab;font-size:12px">Worktic AI · Automatización y ventas con inteligencia artificial</p></td></tr></table></body></html>`

type messageData struct{ Title, Body, Link, Button, Footer string }

func render(d messageData) string {
	t := template.Must(template.New("email").Parse(mailShell))
	var b bytes.Buffer
	_ = t.Execute(&b, d)
	return b.String()
}
func langEN(l string) bool { return strings.HasPrefix(strings.ToLower(l), "en") }
func WelcomeEmail(name, baseURL, locale string) (string, string, string) {
	if name == "" {
		name = ""
	} else {
		name = ", " + name
	}
	d := messageData{Title: "¡Bienvenido a Worktic AI" + name + "!", Body: "Tu cuenta está lista. Desde Worktic AI podrás gestionar clientes, conversaciones, agentes y automatizaciones desde un solo lugar.", Link: baseURL + "/login.html", Button: "Ingresar a Worktic AI", Footer: "Si no creaste esta cuenta, contacta al equipo de soporte."}
	subj := "Bienvenido a Worktic AI"
	if langEN(locale) {
		subj = "Welcome to Worktic AI"
		d = messageData{Title: "Welcome to Worktic AI" + name + "!", Body: "Your account is ready. Manage customers, conversations, agents and automations in one place.", Link: baseURL + "/login.html", Button: "Sign in to Worktic AI", Footer: "If you did not create this account, please contact support."}
	}
	return subj, render(d), d.Title + "\n\n" + d.Body + "\n" + d.Link + "\n" + d.Footer
}
func ResetEmail(name, link, locale string) (string, string, string) {
	d := messageData{Title: "Restablece tu contraseña", Body: "Recibimos una solicitud para restablecer la contraseña de tu cuenta Worktic AI. Este enlace vence en 20 minutos y solo puede usarse una vez.", Link: link, Button: "Crear nueva contraseña", Footer: "Si no solicitaste este cambio, puedes ignorar este correo. Nunca compartas el enlace."}
	subj := "Restablecer contraseña · Worktic AI"
	if langEN(locale) {
		subj = "Reset your password · Worktic AI"
		d = messageData{Title: "Reset your password", Body: "We received a request to reset your Worktic AI password. This one-time link expires in 20 minutes.", Link: link, Button: "Set a new password", Footer: "If you didn't request this, ignore this email. Never share the link."}
	}
	return subj, render(d), d.Title + "\n\n" + d.Body + "\n" + link + "\n" + d.Footer
}
func PasswordChangedEmail(name, baseURL, locale string) (string, string, string) {
	d := messageData{Title: "Tu contraseña fue actualizada", Body: "La contraseña de tu cuenta Worktic AI se cambió correctamente.", Link: baseURL + "/login.html", Button: "Ir a Worktic AI", Footer: "Si no realizaste este cambio, contacta inmediatamente a soporte."}
	subj := "Contraseña actualizada · Worktic AI"
	if langEN(locale) {
		subj = "Your password was updated · Worktic AI"
		d = messageData{Title: "Your password has been updated", Body: "The password for your Worktic AI account has been changed.", Link: baseURL + "/login.html", Button: "Sign in", Footer: "If this wasn't you, contact support immediately."}
	}
	return subj, render(d), d.Title + "\n\n" + d.Body + "\n" + d.Link + "\n" + d.Footer
}

// The following are templates only. Hook them to the actual account/team and billing events.
func TeamInvitationEmail(company, link string) (string, string, string) {
	d := messageData{Title: "Te invitaron a Worktic AI", Body: "Has recibido una invitación para unirte al espacio de trabajo de " + company + ".", Link: link, Button: "Aceptar invitación", Footer: "Si no conoces esta invitación, ignórala."}
	return "Invitación a Worktic AI", render(d), d.Body + "\n" + link
}
func PaymentReceiptEmail(description string) (string, string, string) {
	d := messageData{Title: "Confirmación de pago · Worktic AI", Body: "Registramos tu operación: " + description, Footer: "Consulta el detalle y los comprobantes en tu cuenta de Worktic AI."}
	return "Comprobante Worktic AI", render(d), d.Body + "\n" + d.Footer
}
