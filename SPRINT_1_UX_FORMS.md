# Worktic AI — Sprint 1 UX: formularios y modales

## Cambios aplicados

- Todos los modales ahora respetan un máximo del 90 % de la altura visible.
- El contenido largo se desplaza dentro del modal sin mover toda la aplicación.
- Los botones de acción permanecen visibles mediante un pie fijo durante el desplazamiento.
- Se añadió cierre visible, cierre al pulsar fuera del modal y soporte de `Esc` nativo.
- Se bloquea el desplazamiento del fondo mientras hay un modal abierto.
- Se mejoró el comportamiento en móviles usando `dvh` y áreas seguras.
- Inputs, selects, textareas y columnas dobles se adaptan al ancho disponible.
- Se añadió enfoque automático al primer control disponible para acelerar la operación.
- Se respetan las preferencias de movimiento reducido del sistema.

## Archivos modificados

- `static/styles.css`
- `static/app.js`

## Validación realizada

- JavaScript validado correctamente con `node --check static/app.js`.
- No fue posible ejecutar la compilación Go dentro del entorno de entrega porque el proyecto exige Go 1.24 y el entorno local dispone de Go 1.23.2 sin acceso de red para descargar el toolchain. No se modificó lógica Go.

## Prueba recomendada

1. Abrir cada formulario de creación y edición.
2. Reducir la altura de la ventana.
3. Confirmar que el cuerpo desplaza internamente.
4. Confirmar que Guardar y Cancelar permanecen visibles.
5. Probar en móvil, tablet y escritorio.
