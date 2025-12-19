# ProxyPilot & Droid CLI Troubleshooting Log
**Date:** 2025-12-19
**Subject:** Gemini/Antigravity API Key & Quota Issues

## 1. Issue Summary
The user encountered `[HTTP 400] API key not valid` and `[HTTP 429] Resource Exhausted` errors when using the `droid` CLI with the `gemini-3-pro-preview` model.

## 2. Root Causes Identified

### A. Wrong Model Routing (Primary Cause of Quota Error)
*   **Symbol:** `gemini-3-pro-preview`
*   **Issue:** In ProxyPilot's registry, `gemini-3-pro-preview` is defined as a **Google/Gemini CLI** model, not an Antigravity model. Requests were routing directly to Google using personal CLI keys/quota, which were exhausted (429).
*   **Fix:** Switched Droid configuration to use **`gemini-3-pro-high`** (or `low`), which are the correct aliases for Antigravity-routed models. These are now confirmed working.

### B. Antigravity Account Rotation Failure
*   **Issue:** Even when using Antigravity, only 1 of the 3 available accounts was being utilized.
*   **Cause:** The Auth Selector (`selector.go`) uses a "Strict Fallback" heuristic. If exactly *one* auth file contains a `project_id` field, it is designated as the "Primary" account, and others are ignored until the primary is effectively invalid.
*   **Discovery:** Only the account `truongnamphong8` had a `project_id`.
*   **Fix:** Removed the `project_id` field from the auth file so all 3 accounts are treated as "peers" and rotated via Round Robin.

### C. "API Key Not Valid" Startup Error
*   **Issue:** ProxyPilot server logs showed API key errors on startup.
*   **Cause:** `config.yaml` contained a placeholder line: `api-key: "YOUR_LEGIT_API_KEY_HERE"`.
*   **Fix:** Commented out the invalid line in `config.yaml`.

### D. Droid CLI Config Corruption
*   **Issue:** Droid failed to load with "Failed to parse custom models config".
*   **Cause:** The `config.json` file was saved with a UTF-8 **BOM** (Byte Order Mark) by PowerShell during debugging/editing.
*   **Fix:** Rewrote `config.json` with UTF-8 (No BOM) encoding.

## 3. Preventative Measures & Best Practices

### Model Naming
**ALWAYS** use the Antigravity-specific model IDs to ensure requests route through the shared pool rather than personal quota:
*   ✅ `gemini-3-pro-high`
*   ✅ `gemini-3-pro-low`
*   ✅ `gemini-3-flash` (if available via Antigravity)
*   ✅ `antigravity-claude-sonnet-4-5-thinking`
*   ❌ `gemini-3-pro-preview` (Routes to Google CLI)

### Auth File Management
When adding new Antigravity accounts (`.json` files in `~/.cli-proxy-api/`):
*   Ensure **uniformity**. Either *all* files should have a `project_id`, or *none* should.
*   If mixed, the one with `project_id` will monopolize traffic.

### Configuration
*   Do not leave placeholder values (like "YOUR_KEY_HERE") in `config.yaml`; comment them out.
*   When editing JSON configuration files on Windows, ensure your editor saves as **UTF-8 (No BOM)**.

## 4. Current Configuration (Verified)
*   **Droid Config:** `~/.factory/config.json` updated with correct Antigravity model names.
*   **ProxyPilot Config:** `d:\code\ProxyPilot\config.yaml` cleaned of invalid keys.
*   **Auth Status:** 3 Antigravity accounts active and rotating.

## 5. Issue: Rate Limit Rotation "Stuck" on One Account
**Date:** 2025-12-20
**Subject:** HTTP 429 Handling in Antigravity Executor & Manager

### A. Issue Summary
The user experienced repeated 429 Too Many Requests errors from the *same* account (yuhh0704@gmail.com) despite having multiple healthy backup accounts (nnq2366) available. The rotation logic refused to switch to the backup, effectively blocking the user.

### B. Root Causes
1.  **Missing Retry-After Parsing:** The AntigravityExecutor was receiving a Retry-After header (e.g., 1 hour) from the upstream API but was ignoring it. It only returned a generic error message.
    *   **Effect:** The AuthManager used a default backoff of only **1 second**.
2.  **Model State Not Updated:** When the global Auth object was marked as "Rate Limited", the specific ModelState for the requested model (e.g., claude-opus-4-5-thinking) remained "Active".
    *   **Effect:** The RoundRobinSelector checked the *model* state, saw it was valid, and re-selected the same "Primary Fallback" account (due to strict fallback logic preferences) immediately after the 1-second backoff expired.

### C. The Fix
1.  **Updated AntigravityExecutor:** Added logic to parse the Retry-After header (supporting both integer seconds and HTTP date formats) and pass it to the statusErr struct.
2.  **Updated AuthManager (MarkResult):** Modified the result handling to explicitly update the ModelState with the error and cooldown info, not just the parent Auth object.
3.  **Updated Selector (isAuthBlockedForModel):** Added a defensive check to respect NextRetryAfter timestamps even if the Unavailable boolean flag wasn't explicitly set, ensuring blocked models are strictly filtered out.

### D. Result
System now respects long cooldowns (e.g., 3 hours) and correctly skips the blocked "Primary" account to use the "Secondary" backups like nnq2366.
