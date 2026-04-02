from pathlib import Path
import unittest

from prototype.core import LocalAssistant, WorkspaceManager, is_video_url, scan_markdown_tasks

FIXTURE_WORKSPACE = Path(__file__).resolve().parent / "fixtures" / "workspace"


class CoreTests(unittest.TestCase):
    def test_scan_markdown_tasks(self) -> None:
        tasks = FIXTURE_WORKSPACE / "tasks"
        outcome = scan_markdown_tasks([tasks], workspace_root=FIXTURE_WORKSPACE, known_mtimes={})
        self.assertEqual(outcome.pending_count, 1)
        self.assertEqual(outcome.completed_count, 1)
        self.assertEqual(len(outcome.items), 2)

    def test_video_detection(self) -> None:
        self.assertTrue(is_video_url("https://www.youtube.com/watch?v=abc123"))
        self.assertTrue(is_video_url("https://bilibili.com/video/BV1xx"))
        self.assertFalse(is_video_url("https://example.com/article"))

    def test_assistant_summary(self) -> None:
        assistant = LocalAssistant()
        summary = assistant.summarize("第一行。第二行。第三行。")
        self.assertIn("摘要", summary)
        self.assertIn("核心", summary)

    def test_workspace_demo_content(self) -> None:
        manager = WorkspaceManager(FIXTURE_WORKSPACE)
        path = manager.ensure_demo_content()
        self.assertTrue(path.exists())


if __name__ == "__main__":
    unittest.main()
