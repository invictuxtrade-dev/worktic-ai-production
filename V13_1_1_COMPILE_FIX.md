# V13.1.1 Compile Fix

Corrección puntual de compilación para Windows.

## Error resuelto

`normalizePhone redeclared in this block`

La función `normalizePhone` estaba declarada dos veces dentro de `main.go`. Se eliminó la declaración duplicada y se conservó una sola implementación compartida.

## Instalación

1. Extraer en una carpeta nueva.
2. Ejecutar `DESBLOQUEAR_WINDOWS.bat`.
3. Ejecutar `INSTALAR_DEPENDENCIAS_WINDOWS.bat`.
4. Ejecutar `INICIAR_WINDOWS.bat`.
