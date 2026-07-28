# Hotfix de vistas globales

Se corrigió una regla CSS que forzaba `.inbox-view` a `display:block` incluso cuando la sección no tenía la clase `active`.

La bandeja de conversaciones ahora solo se muestra cuando el menú activo es `inbox`; el resto de módulos vuelve a renderizarse de forma independiente.
