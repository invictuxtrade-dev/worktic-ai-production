#!/usr/bin/env python3
"""Offline supplier coverage audit V27.31. No DB, cloud, network, keys or provider calls.
Outputs JSON ONLY with coverage not certified; never makes a green launch claim.
"""
import csv
import json
import pathlib
import sys
from collections import Counter

ROOT=pathlib.Path(__file__).resolve().parents[2]
SOURCE=ROOT/'audits'/'MATRIZ_COSTOS_TODOS_MODULOS_V2722.csv'


def audit(path=SOURCE):
    with open(path, encoding='utf-8-sig', newline='') as handle:
        rows=list(csv.DictReader(handle))
    required={'group','cost_driver','measurement','provider_cost_verified','commercial_status'}
    if not rows or not required.issubset(rows[0]):
        raise ValueError('La matriz de costos no contiene columnas esperadas')
    seen=set()
    uncovered=[]
    for row in rows:
        g=row['group']
        if g in seen:
            raise ValueError('grupo duplicado: '+g)
        seen.add(g)
        evidence=(row.get('provider_cost_verified') or '').strip().lower()
        if evidence not in ('yes','verified','si','sí'):
            uncovered.append({'module':g,'cost_driver':row['cost_driver'],'evidence':evidence or 'unknown','status':'provider_cost_not_verified'})
    return {'release':'V27.31','module_groups':len(rows),'provider_costs_not_verified':len(uncovered),
            'gaps':uncovered,'public_launch_approved':False,
            'pricing_profit_statement':'NOT VERIFIED: provider invoice, measured infrastructure, settlement and actual FX missing'}


def main():
    report=audit()
    dest=ROOT/'audits'/'COSTEO_COBERTURA_V2731.json'
    dest.write_text(json.dumps(report,ensure_ascii=False,indent=2)+'\n',encoding='utf-8')
    print(json.dumps({'groups':report['module_groups'],'not_verified':report['provider_costs_not_verified'],'launch_approved':False,'output':str(dest)},ensure_ascii=False))
    return 0

if __name__=='__main__':
    sys.exit(main())
