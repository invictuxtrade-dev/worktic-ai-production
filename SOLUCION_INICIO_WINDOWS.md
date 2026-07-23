# Corrección de inicio en Windows — V13.0.4

Esta versión corrige dos problemas diferentes:

1. Windows podía bloquear los archivos `.bat` extraídos de un ZIP descargado.
2. El iniciador ejecutaba el binario usando solo su nombre y abría el navegador antes de confirmar que el servidor había iniciado.

## Instalación recomendada

1. Extrae el ZIP en una carpeta nueva y corta, por ejemplo `C:\Worktic\V13`.
2. Ejecuta `DESBLOQUEAR_WINDOWS.bat` una sola vez.
3. Ejecuta `INSTALAR_DEPENDENCIAS_WINDOWS.bat`.
4. Ejecuta `INICIAR_WINDOWS.bat`.

El nuevo iniciador utiliza la ruta absoluta del ejecutable:

`worktic-ai-v13.exe`

El navegador se abre después de iniciar el proceso, evitando mostrar `ERR_CONNECTION_REFUSED` por abrirse demasiado pronto.

## Si Windows todavía bloquea el archivo

Haz clic derecho sobre el ZIP original, abre **Propiedades**, marca **Desbloquear**, aplica el cambio y vuelve a extraerlo.
