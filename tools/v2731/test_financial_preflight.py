import tempfile
import unittest
from pathlib import Path
from financial_preflight import audit


class FinanceAuditV2731(unittest.TestCase):
    def test_inventory_is_unverified_until_source_evidence(self):
        output=audit()
        self.assertGreaterEqual(output['module_groups'],31)
        self.assertGreater(output['provider_costs_not_verified'],0)
        self.assertFalse(output['public_launch_approved'])

    def test_no_implicit_verified_costs(self):
        with tempfile.TemporaryDirectory() as td:
            csvpath=Path(td)/'test.csv'
            csvpath.write_text('group,cost_driver,measurement,provider_cost_verified,commercial_status\nmeta,message,messages,no,pending\n',encoding='utf-8')
            output=audit(csvpath)
            self.assertEqual(output['provider_costs_not_verified'],1)
            self.assertFalse(output['public_launch_approved'])

    def test_duplicate_cost_group_rejected(self):
        with tempfile.TemporaryDirectory() as td:
            csvpath=Path(td)/'test.csv'
            csvpath.write_text('group,cost_driver,measurement,provider_cost_verified,commercial_status\nmeta,message,messages,no,pending\nmeta,message,messages,yes,done\n',encoding='utf-8')
            with self.assertRaises(ValueError):
                audit(csvpath)

if __name__=='__main__':unittest.main()
