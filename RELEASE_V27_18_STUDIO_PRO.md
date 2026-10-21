# WorkticAI V27.18 — Worktic Studio Pro

## Source of truth

This is an additive release built strictly on the WorkticAI V27.17 project ZIP, itself based on the user's V27.16.1 production backup (Git commit 09e37d4). The commercial / Partner subsystem, PostgreSQL schema, Redis, Auth, WhatsApp, Copilot and existing Social Hub API are not replaced or reset. No existing project files are removed.

## What's new

- **Workspace**: automatically collapses the main WorkticAI sidebar to icons on entering Studio, with controls to expand it or enable distraction-free focus; restores normal behavior when leaving.
- **Social formats**: preset profiles for TikTok, Instagram Reels / Stories / Post, YouTube Shorts / landscape, Facebook feed / Reels, WhatsApp Status and LinkedIn. The presets choose aspect ratios and supply editable safe-area advice; they do not imply platform publication certification.
- **Video timeline**: drag to reorder clips; drag media from the asset grid onto the timeline; split, duplicate or delete a selected clip; move/rescale timed text overlays; timeline zoom; drag/retrim narrated segments; captions and a separate voice lane. A short dissolve-to-black transition and basic color filters are supported on the canvas/WebM export.
- **Audio**: six original genre/tempo presets synthesized locally into WAV (corporate, lo-fi, electro, cinematic, ambient, latin); uploaded to the existing tenant media gallery and looped while editing/exporting. This is procedural instrumental music, **not a commercial-song library or an AI music model**.
- **Narration**: an authenticated server endpoint `POST /api/studio/narration` calls OpenAI speech generation, with allowlisted voices and speed, stores the MP3 as a tenant asset and adds it to a voice track. Optional subtitles are distributed in estimated time intervals; timing should be reviewed by a human before publication. Generated voice is visibly disclosed as synthetic.
- **Auto montage**: creates an editable sequence from media already uploaded by the user. Existing Copilot storyboard/text/image functionality remains. **This is not native text-to-video generation.**
- **Image Studio**: keeps editable layers, images, texts, color/opacity/position; adds quick duplicate, center, promo badge controls and existing templates.
- **Publishing**: PNG / WebM export stored in the same Multimedia Gallery used by Social Hub.

## Limitations / reality checks

1. Current video export remains **WebM** via Canvas + MediaRecorder, up to 60 seconds / 50 MB. Native MP4, long form multi-track compositing, object tracking, chroma key, professional color grading, precise waveform editing, per-frame keyframes and true CapCut feature parity are **not included**. MP4 requires a server-side FFmpeg pipeline or comparable production renderer.
2. **There is no true text-to-video model integration.** The prior OpenAI Videos API / Sora 2 endpoint was discontinued in September 2026. A separately supported provider/service would be needed, with quotas, moderation, asynchronous job state and cost governance.
3. OpenAI speech is currently integrated with the `/v1/audio/speech` `gpt-4o-mini-tts` API, which is scheduled for shutdown on January 6, 2027. Prepare migration to a supported Realtime-based speech workflow; the model can be configured via `WORKTIC_STUDIO_TTS_MODEL`, though changing it to a Realtime model alone is not sufficient.
4. OpenAI calls consume actual API credits. Speech and image use existing `studio_ai_usage_v2711` tenant/day quotas. No secret API key is exposed to the browser.
5. Music loops are synthesized WAV, not preloaded copyrighted tracks; uploaded files remain 20 MB maximum for audio. TTS mp3 files count towards storage.
6. This release must still pass `go test ./...` and Docker compilation **with Go 1.26** before production, and functional staging tests with PostgreSQL/Redis/OpenAI and genuine media. The build environment available here only has Go 1.23, so the Go test command could not run here. Browser tests use simulated responses and do not prove provider API availability.

## Changed/additive files

- Modified: `VERSION.txt`, `main.go`, `studio_v2711.go`, `static/app.html`, `static/worktic-studio-v2711.js`, `static/worktic-studio-v2711.css`.
- Added: `studio_v2718.go`, `static/worktic-studio-v2718.js`, `RELEASE_V27_18_STUDIO_PRO.md`.
- No project source files from the original V27.17 deleted. No `.git` included.

## Safe deploy for the Windows + GitHub + Render environment

1. Take backups of PostgreSQL and `/var/data` and preserve the last successful Render deployment.
2. Confirm your current `origin/main` and Render deployment commit reflect V27.17. This ZIP is based on V27.17; if another production commit exists, integrate the changed files into that newer commit rather than blindly replacing it.
3. Create a separate Git worktree or a local clone from the actual deployed base. Copy the changed/new files listed above into that worktree, leaving all other project files and secrets untouched.
4. Verify `node --check static/worktic-studio-v2711.js`, `node --check static/worktic-studio-v2718.js`, `node --check static/app.js`, `git diff --check`, `go test ./...`, `go build ./...` and Docker build using Go 1.26.
5. Test on a separate staging Render instance with its own PostgreSQL database and disk, and confirm project save/load, uploads, MP3 voice generation, PNG/WebM export with sound, mobile, Social Hub and existing Partners.
6. Push the tested commit through your usual GitHub deployment workflow, manually deploy the commit on Render, confirm `/healthz` and `/readyz`, and verify the sidebar and Studio tools while authenticated.
7. Do **not** run SQLite-to-PostgreSQL migration again. No new schema migration is needed beyond the tables already introduced in V27.17.

## Browser QA performed (local mocks)

- Worktic Studio enters collapsed sidebar mode and allows focus/expansion.
- Ratio presets switch between 9:16 and 4:5; the format note updates.
- Image editor saves and exports a PNG; the mock media gallery receives it.
- Video sequence can be auto-created from gallery items, duplicated, split, and zoomed.
- Original instrumental WAV is synthesized, uploaded and assigned to a music track.
- Narration endpoint is called with a mocked service response; voice and estimated captions are added and saved.
- WebM export with music/voice sources completes and uploads via the mocked gallery.
- Browser JavaScript errors: none. Horizontal overflow at 390px: none.

## Rollback

Roll back the code deployment to the previously working Render commit if needed. Code rollback does not undo changes made to PostgreSQL or `/var/data`. This release adds no database tables and does not modify previous billing or Partner tables.
