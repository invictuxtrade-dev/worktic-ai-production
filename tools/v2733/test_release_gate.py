"""Offline gate must deny incomplete/forged/stale staging evidence."""
import unittest, json, tempfile, pathlib, sys
sys.path.insert(0,str(pathlib.Path(__file__).resolve().parent))
from release_gate import audit, safe_evidence_path, QA_IDS
class Gate(unittest.TestCase):
 def test_missing_evidence_is_blocked(self):
  self.assertTrue(audit(pathlib.Path('/no/such/file.json')))
 def test_template_is_blocked(self):
  p=pathlib.Path(__file__).parent/'STAGING_EVIDENCE_TEMPLATE.json'
  self.assertTrue(audit(p))
 def test_reject_path_traversal(self):
  self.assertIsNone(safe_evidence_path('audits/evidence_v2733/../../VERSION.txt'))
 def test_reject_arbitrary_files(self):
  self.assertIsNone(safe_evidence_path('VERSION.txt'))
 def test_qa_ids(self):
  self.assertEqual(QA_IDS,[f'QA-{i:02d}' for i in range(1,17)])
if __name__=='__main__':unittest.main()
