#!/usr/bin/env python3
"""Offline checks of V27.28 integration. Does not contact user DB or vendors."""
from pathlib import Path
import csv,re,sys
root=Path(__file__).resolve().parents[2]
checks={
 'main.go': ['initBillingLifecycleV2728Schema(db)','/api/billing/v2728/preview','/api/admin/billing/v2728/audit','billingClaimTxV2728(tx','billingTransitionV2728('],
 'commercial_credits_v2721.go':['billingClaimTxV2728(tx','studio_topup'],
 'finance_v2725.go':['billingTransitionV2728('],
 'admin_full.go':['billingCourtesyAllowedV2728','billingLifecycleV2728Enabled()'],
 'static/app.js':['reviewLifecycle','wtBillingPolicyV2728'],
 'commercial_validation_v2727.go':['lifecycle_v2728'],
 'billing_lifecycle_v2728.go':['CREATE TABLE IF NOT EXISTS billing_payment_claims_v2728','PRIMARY KEY(network,tx_hash)'],
}
for f,parts in checks.items():
 text=(root/f).read_text(encoding='utf-8')
 for part in parts:
  if part not in text: raise SystemExit(f'FALLO {f}: {part}')
rows=list(csv.DictReader((root/'audits/ACEPTACION_RECORRIDOS_V2728.csv').open(encoding='utf-8')))
assert len(rows)>=18
assert all(r['estado_en_este_entorno']!='certificado_produccion' for r in rows)
print('OK integración estática: 7 archivos de código,',len(rows),'recorridos auditables. NO certifica producción.')
