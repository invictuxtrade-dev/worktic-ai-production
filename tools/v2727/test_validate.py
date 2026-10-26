import unittest
from decimal import Decimal as D
from pathlib import Path
from validate import audit, scenario,PLANS,FEATURES,PROVIDER_ACTIONS
ROOT=Path(__file__).resolve().parents[2]
class Tests(unittest.TestCase):
 def test_free_no_provider(self):self.assertEqual(PLANS['free']['studio']+PLANS['free']['core'],0)
 def test_all_plans_have_feature_matrix(self):self.assertEqual(set(PLANS),set(FEATURES))
 def test_features_monotonic(self):
  self.assertTrue(FEATURES['free'].issubset(FEATURES['personal']))
  self.assertTrue(FEATURES['personal'].issubset(FEATURES['business']))
  self.assertTrue(FEATURES['business'].issubset(FEATURES['enterprise']))
 def test_commission_ceiling(self):
  with self.assertRaises(ValueError): scenario('business',20,51,None)
 def test_discount_ceiling(self):
  with self.assertRaises(ValueError): scenario('business',81,20,None)
 def test_null_fx_no_fake_margin(self):self.assertIsNone(scenario('enterprise',20,50,None)['remaining_usd'])
 def test_illustrative_margin(self):
  r=scenario('personal',20,50,D('1'))
  self.assertEqual(r['remaining_usd_before_other_costs'],'7.75')
 def test_zero_partners_more_capacity(self):
  a=scenario('enterprise',20,0,D('1'))
  b=scenario('enterprise',20,50,D('1'))
  self.assertGreater(D(a['remaining_usd_before_other_costs']),D(b['remaining_usd_before_other_costs']))
 def test_fx_warnings(self):
  with self.assertRaises(ValueError):scenario('personal',0,0,D('0'))
 def test_all_quotes_within_bound(self):
  for name,credits,cents in PROVIDER_ACTIONS:self.assertLessEqual(cents*2,credits*5,name)
 def test_historic_routes(self):self.assertEqual(audit(ROOT)['newly_missing_routes'],[])
 def test_launch_never_auto_approved(self):self.assertFalse(audit(ROOT)['public_launch_approved'])
 def test_report_contains_all_plans(self):
  data=audit(ROOT);self.assertEqual({x['plan'] for x in data['scenarios']},set(PLANS))
 def test_actual_plan_cap_in_writer(self):
  s=(ROOT/'commercial_core_ai_v2721.go').read_text();self.assertIn('commercialCoreCapForPlan(plan, billingCap)',s)
if __name__=='__main__':unittest.main()
