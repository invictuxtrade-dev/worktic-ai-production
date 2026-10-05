# Configuración Meta para WorkticAI V17

## 1. Meta Developers
Usa la misma Meta App empresarial de WorkticAI. Debe estar asociada al negocio verificado correcto.

## 2. WhatsApp / Embedded Signup
En la configuración de WhatsApp Embedded Signup / Facebook Login for Business crea una configuración para el onboarding de clientes. Copia el **Configuration ID**.

En Render define:

```env
META_APP_ID=...
META_APP_SECRET=...
META_WHATSAPP_CONFIG_ID=...
META_GRAPH_VERSION=v25.0
WHATSAPP_VERIFY_TOKEN=...
```

Opcional para el modelo Tech Provider:

```env
META_SYSTEM_USER_ACCESS_TOKEN=...
```

## 3. Dominio
`BASE_URL` debe ser HTTPS y corresponder al dominio público real de WorkticAI. Añade ese dominio en la configuración permitida de la Meta App/Facebook Login cuando Meta lo solicite.

## 4. Webhook de WhatsApp
Callback de Worktic:

```text
https://TU-DOMINIO/webhooks/meta/whatsapp
```

El Verify Token es el valor de `WHATSAPP_VERIFY_TOKEN` en Render.

## 5. Permisos / App Review
El acceso de producción depende del modelo configurado en Meta y de los permisos/accesos aprobados. Para Embedded Signup Meta documenta Advanced Access para `business_management` y `whatsapp_business_management`; la mensajería Cloud utiliza también los permisos correspondientes de WhatsApp Business Platform.

## 6. Prueba en Worktic
1. Canales → Nueva conexión → **WhatsApp Business Cloud (Oficial)**.
2. Crear el canal.
3. Pulsar **Conectar**.
4. Introducir el PIN de seis dígitos cuando corresponda.
5. Pulsar **Continuar con Meta**.
6. Seleccionar/crear negocio, WABA y número dentro del flujo de Meta.
7. Worktic intercambia el código en backend, valida los activos y suscribe el WABA al webhook.
8. Comprobar **Probar conexión** y **Plantillas**.

## Fallback
La conexión manual por WABA ID + Phone Number ID + Access Token sigue disponible en un desplegable avanzado. Esto facilita diagnóstico y migraciones sin bloquear Embedded Signup.
