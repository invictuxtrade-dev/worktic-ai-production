# WorkticAI V27.35.8 — Formulario Agentes IA, KPIs y planes publicados

Base exacta: V27.35.7. Instalador con verificacion de hashes y cambios Git.

## Agentes IA
- Tipos traducidos, manteniendo codigos internos compatibles.
- Herramientas y canales con seleccion multiple; los canales se asignan realmente desde Canales.
- Campos claramente obligatorios y opcionales. El objetivo se exige al activar un agente.
- Preserva codigos de herramientas/canales preexistentes que no figuren en el catalogo visual.
- Guardado con respuesta de error visible y posibilidad de reintento; sin listeners globales acumulados.

## Resumen
- KPIs Ingresos (anteriormente Revenue), Ventas, Leads y Conversaciones con definiciones visibles y tooltip.
- No modifica los calculos, las métricas del backend ni los valores históricos.

## Planes publicos
- La pagina publica muestra solo los planes activos recibidos del servidor. Ya no exige exactamente 4 planes.
- El endpoint mantiene los importes del backend y excluye planes cuyos beneficios no se puedan validar.
- Administracion Worktic > Planes: acciones Publicar en web / Ocultar de la web.
- Deshabilitar un plan detiene las nuevas compras: suscripciones existentes mantienen su vigencia.
- Free no puede deshabilitarse con la accion rapida porque es el plan de respaldo.
- Si no hay planes publicables, la web muestra una nota de no disponibilidad sin inventar precios.

## Limites y comprobaciones pendientes
- Ejecutar node --check en los JS modificados, go test -count=1 ./..., go build ./..., git diff --check.
- Validar en dos cuentas, interfaz movil y escritorio; comprobar publicacion del plan y checkout usando una cuenta de pruebas.
- Si /api/public/offer/v2734 falla por otro problema de DB, recopilar el codigo HTTP y los logs de Render; este parche no reconstruye datos ni inventa reglas comerciales.
- No reemplazar el repositorio completo sobre produccion: instalar el parche y comprobar antes de push.
