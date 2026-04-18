# Beta Release Readiness

- Windows x64 `setup.exe` builds from the workspace root with `pnpm build:bundle`.
- The packaged desktop host launches the bundled Go local service on startup.
- First launch initializes the user data directory instead of reading repository-relative paths.
- Notes, mirror, security, and settings all render seeded content on the first dashboard visit.

## Remaining Manual Checks

1. Install on a clean Windows profile.
2. Verify `CialloClaw_<version>_x64-setup.exe` starts without the repository present.
3. Confirm the first-launch seed data only imports into the user data directory.
