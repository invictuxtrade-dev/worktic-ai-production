# Corrección de persistencia del calendario

- Toda publicación con fecha y hora se normaliza automáticamente a estado `scheduled`, salvo cuando la acción sea publicar inmediatamente.
- La validación se aplica tanto en frontend como en backend.
- La edición de publicaciones conserva y vuelve a guardar la programación completa.
- El calendario muestra cualquier publicación que tenga `scheduled_at`, evitando ocultarla por un estado inconsistente heredado.
- Se valida que la fecha enviada tenga formato RFC3339.
