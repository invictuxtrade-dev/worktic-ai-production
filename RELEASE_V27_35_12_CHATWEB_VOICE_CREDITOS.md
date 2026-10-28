# WorkticAI V27.35.12 - Integración comercial y saldos combinados

Base: V27.35.11. Sin migraciones, sin cambios a las tablas de planes y sin alterar las capacidades ni los precios vigentes.

## Cambios
- Página pública: Chat Web como característica dentro del cupo TOTAL de conexiones vigente; comparación con indicador ✓ cuando el plan dispone de conexiones. No se promete cupo adicional. Se mantiene el límite técnico de hasta 2 canales Chat Web por empresa.
- Nueva sección comercial premium de Llamadas IA exactamente debajo de Worktic Studio. Disposición responsive y versión inglés/español; se advierte la dependencia del proveedor y del saldo de voz.
- Cabecera de backoffice: conserva el botón y su aspecto; muestra total de los saldos Studio + Llamadas IA verificados. Si la lectura de voz falla, el total NO se presenta como completo: muestra temporalmente el valor Studio con título aclaratorio. Se consulta a `/api/voice/v2735/overview` autenticado y se evita duplicar solicitudes.
- Menú lateral: eliminada la etiqueta de usuario en el pie sin modificar enlaces ni navegación.

## Pruebas manuales necesarias
1. Refrescar página comercial y verificar Chat Web dentro de los planes y la tabla comparativa sin alterar precios.
2. Ver sección Llamadas IA inmediatamente después de Studio en escritorio y móvil; sin scroll horizontal.
3. Iniciar sesión en empresa con saldos Studio y Voz conocidos, verificar que el número de la cabecera sea su suma; comparar con las dos tarjetas en Billetera de Créditos.
4. Con endpoint de Voz no disponible, confirmar que el título indique saldo parcial y la cabecera no atribuya falsamente el saldo Studio a Voz.
5. Confirmar que los enlaces Documentación, idioma, Cerrar sesión y navegación permanecen operativos.

El saldo combinado es una presentación; no transfiere créditos entre servicios, ni cambia los ledgers. No modifica ni permite comprar llamadas salientes.
