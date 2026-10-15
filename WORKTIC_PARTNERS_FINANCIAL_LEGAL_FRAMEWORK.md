# Worktic Partners — Marco financiero y operativo propuesto

> Documento interno de diseño. No reemplaza asesoría jurídica, tributaria, contable ni regulatoria. El programa público se mantiene **informativo / pre-lanzamiento** hasta completar la validación correspondiente.

## 1. Regla económica central

Worktic Partners debe pagar por **ventas reales, servicio de adopción verificable, renovación y mentoría elegible**, nunca por el simple registro o incorporación de personas.

### Ingreso Neto Elegible (INE)

La base recomendada para calcular comisiones es el ingreso del servicio efectivamente cobrado y reconocido en el periodo, después de:

- descuentos y créditos comerciales;
- impuestos indirectos cobrados al cliente;
- reembolsos y chargebacks;
- costos del medio de pago / adquirencia;
- cargos variables de terceros definidos expresamente en la política del programa.

**Fórmula conceptual:**

`INE = ingreso reconocido - descuentos - impuestos indirectos - refunds/chargebacks - costos de cobro - cargos variables definidos`

Las comisiones se calculan sobre el INE y no necesariamente sobre el precio publicado del plan.

### Guardrail financiero antes de publicar el plan

El 50% no debe usarse como objetivo de payout. Antes de activar el programa se debe comprobar la economía unitaria de **cada plan y ciclo** con una prueba de estrés. La regla recomendada es:

`Pool contractual máximo <= min(50% del INE esperado, margen disponible después de costos y reserva empresarial)`

Donde el margen disponible se calcula, antes de fijar los porcentajes contractuales, como:

`Margen disponible = ingreso reconocido - costos variables no incluidos en INE - reserva de soporte/servicio - contribución mínima requerida por WorkticAI`

La empresa debe definir esos costos y la contribución mínima con datos reales de IA, mensajería, infraestructura, soporte, cobro, fraude, impuestos propios y demás costos relevantes. **Este control se realiza antes de ofrecer el plan**; no debe utilizarse para reducir retroactivamente una comisión ya causada bajo un contrato vigente.

Prueba mínima por plan:

- escenario de uso normal;
- escenario de uso alto dentro de los límites del plan;
- escenario de máximo payout elegible;
- anual con 20% de descuento y reconocimiento mensual;
- refund/chargeback y mora;
- aumento razonable de costos de IA, mensajería y adquirencia.

## 2. Techo de distribución por suscripción

La suma máxima combinada prevista es **50% del INE** de cada periodo reconocido:

| Concepto | Máximo |
|---|---:|
| Comisión directa | 34% |
| Nivel 2 | 5% |
| Nivel 3 | 3% |
| Adopción | 4% |
| Renovación | 2% |
| Mentoría | 2% |
| **Techo absoluto** | **50%** |

El 50% es un **límite**, no un payout automático. Los porcentajes sin beneficiario elegible no se comprimen ni se redistribuyen: permanecen en WorkticAI.

Si, solo para ilustrar el techo, el INE fuera igual al precio reconocido completo:

| Plan | Mensual reconocido | Techo ecosistema 50% | Anual reconocido por mes | Techo anual por mes | Techo anual 12 meses |
|---|---:|---:|---:|---:|---:|
| Personal | 25 USD | 12,50 USD | 20 USD | 10 USD | 120 USD |
| Negocio | 75 USD | 37,50 USD | 60 USD | 30 USD | 360 USD |
| Empresa | 150 USD | 75 USD | 120 USD | 60 USD | 720 USD |

Estos son techos matemáticos del ecosistema, no lo que recibe una sola persona ni una garantía de que el INE equivalga al precio publicado.

## 3. Worktic Momentum

| Rango | Directos activos / adopción | Directa | N2 | N3 |
|---|---:|---:|---:|---:|
| Spark | Inicio | 25% | — | — |
| Pulse | 3 / 2 | 28% | — | — |
| Orbit | 8 / 6 | 30% | 3% | — |
| Apex | 18 / 14 | 32% | 4% | 2% |
| Infinity | 35 / 28 | 34% | 5% | 3% |

Reglas propuestas:

- evaluación mensual;
- 90 días de protección de rango después de una caída;
- desde Orbit, puerta de retención para cohortes con historial suficiente;
- no hay requisito de reclutamiento para ascender;
- un Partner puede llegar a Infinity solo con cartera directa y adopción válida.

## 4. Separación producto / derecho a comisionar

Para reducir el riesgo de que una compra sea interpretada como pago por el derecho a participar, la arquitectura recomendada es:

- **alta como Partner sin costo**, mediante contrato comercial y validaciones aplicables;
- la licencia WorkticAI sigue siendo un producto SaaS independiente;
- una licencia de pago puede habilitar funciones adicionales de Partner Autopilot según el plan, pero **no compra el derecho a recibir comisiones**;
- las comisiones nacen de ventas elegibles y de servicios de adopción/mentoría verificables.

Esta decisión debe validarse con abogado especializado antes del lanzamiento.

## 5. Planes anuales

Los planes anuales se cobran por adelantado, pero la comisión recomendada se **reconoce mes a mes** según la prestación del servicio.

Ejemplo plan Negocio anual:

- precio anual público: 720 USD;
- ingreso reconocido de referencia: 60 USD/mes antes de ajustes de INE;
- el Partner no recibe 12 meses de comisión el día de la compra;
- si existe reembolso, cancelación o chargeback, se detiene el reconocimiento futuro y se aplican reversos cuando corresponda.

Esto reduce riesgo de caja y alinea el payout con la obligación de servicio.

## 6. Bonos

### Adopción — 4%

- se asigna a un único implementador elegible;
- exige Worktic Impact Score >= 70;
- no se paga por instalar una cuenta sin uso real;
- si el vendedor también fue el implementador, puede recibir ambos conceptos siempre que el contrato lo permita y el techo global no se exceda.

### Renovación — 2%

- corresponde al vendedor directo elegible;
- no aplica en la primera compra/ciclo;
- requiere continuidad del cliente y ausencia de refund/chargeback del periodo.

### Mentoría — 2%

- un único mentor de registro;
- requiere acompañamiento documentado;
- no se paga solo por estar arriba en una estructura.

## 7. Co-Sell

Co-Sell divide la comisión directa existente. **No suma una nueva comisión.**

Ejemplo Spark, plan Empresa 150 USD con base elegible = 150 USD:

- pool directo 25% = 37,50 USD;
- referidor 70% = 26,25 USD;
- asesor 30% = 11,25 USD.

Si el rango directo es diferente, el 70/30 se aplica al pool directo correspondiente a ese rango.

## 8. Worktic Impact Score

Puntaje de 0 a 100, adaptable al plan:

| Indicador | Puntos |
|---|---:|
| Configuración útil | 20 |
| CRM con actividad real | 20 |
| Canales conectados y utilizados | 20 |
| Agentes / automatizaciones operativas | 20 |
| Continuidad y actividad comercial | 20 |
| **Total** | **100** |

Umbral propuesto para adopción: **70 puntos**.

La medición debe excluir eventos artificiales, autosimulaciones y actividad generada únicamente para desbloquear bonos.

## 9. Payouts y control de riesgo

Recomendación operativa:

1. corte mensual;
2. periodo de validación/reversa antes de liberar pagos;
3. saldo pendiente vs. saldo aprobado;
4. compensación automática de chargebacks contra futuros pagos;
5. KYC/identidad y datos tributarios antes del primer payout;
6. trazabilidad completa por venta, periodo, concepto, beneficiario y fórmula;
7. no pagar porcentajes huérfanos;
8. no permitir autorreferencias ni ventas circulares;
9. bloquear coincidencias sospechosas de titular, medio de pago, dispositivo o empresa cuando corresponda;
10. payout solo sobre ingreso reconocido y cobrado.

## 10. Simulaciones de referencia

Los siguientes cálculos suponen, únicamente para ilustrar la matemática, que el INE coincide con el precio del plan.

### 1 cliente Negocio de 75 USD · Spark

- directa 25%: 18,75 USD;
- adopción 4%: 3,00 USD;
- renovación 2%: 1,50 USD (no aplica en el primer ciclo);
- máximo personal posterior, si es elegible para ambos bonos: 23,25 USD/mes.

### 10 clientes Negocio de 75 USD · Orbit

- directa 30%: 225 USD;
- adopción 4% sobre 10 clientes elegibles: 30 USD;
- renovación 2% sobre 10 renovaciones elegibles: 15 USD;
- total ilustrativo: 270 USD/mes.

### 35 clientes Negocio de 75 USD · Infinity

- directa 34% + adopción 4% + renovación 2% = 40%;
- 35 x 75 x 40% = 1.050 USD/mes;
- no incluye N2, N3 ni mentoría;
- no es una promesa ni ingreso típico.

## 11. Marco colombiano que debe revisarse antes del lanzamiento

La estructura propuesta de niveles puede encajar dentro de la definición de actividad multinivel cuando concurren incorporación de vendedores, compensación por ventas de la red y coordinación de una red comercial. Por eso el lanzamiento debe revisarse bajo, entre otras normas y lineamientos:

- Ley 1700 de 2013;
- Decreto 24 de 2016 / Decreto 1074 de 2015;
- Circular Básica Jurídica de la Superintendencia de Sociedades, Capítulo IX;
- Ley 1480 de 2011 y obligaciones de comercio electrónico / información al consumidor;
- obligaciones mercantiles, tributarias, contables, de protección de datos y prevención de fraude que correspondan.

Puntos mínimos a validar:

- sociedad habilitada para la actividad si corresponde;
- registro mercantil y objeto/actividad;
- oficina física abierta al público cuando la regulación resulte aplicable;
- contrato escrito con cada vendedor independiente;
- plan de compensación disponible antes de la vinculación y permanentemente en la web;
- forma, periodicidad y requisitos de pago;
- derechos de terminación y ausencia de cláusulas de permanencia/exclusividad prohibidas;
- compensación causalmente vinculada a ventas/servicios elegibles;
- publicidad de ingresos clara, verificable y sin promesas de rentabilidad.

## 12. Decisión recomendada

**No activar todavía payouts ni inscripción automática de Partners.**

La V27.11 publica únicamente la página informativa, el simulador educativo y la arquitectura de producto. El siguiente paso es validar contrato/plan con asesor jurídico y financiero y, una vez aprobado, implementar Partner Center en el CRM con ledger de comisiones, atribución, KYC, reversos y auditoría.
