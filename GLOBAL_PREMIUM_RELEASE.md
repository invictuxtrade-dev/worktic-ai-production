# Worktic AI — Global Premium Release

Esta entrega integra en un solo proyecto:

- Aplicación Worktic AI Premium.
- Landing pública bilingüe ES/EN.
- Social Hub multitenant y analítica avanzada.
- Conectores oficiales preparados.
- Documentación HTML5 completa en `static/docs`.
- Acceso público mediante `https://workticai.com/docs/`.
- Botón de documentación desde la landing y desde la aplicación.

## Ruta de documentación

El servidor Go publica la carpeta `static` como raíz. Al desplegar este proyecto, la documentación queda disponible en:

```text
/docs/
```

En producción:

```text
https://workticai.com/docs/
```

## Importante

Usa la barra final `/docs/` para asegurar que los recursos relativos de la documentación se carguen correctamente.
