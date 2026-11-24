"""Five semantic decisions, exact pytest exit 1 and named invariant controls."""

import hashlib
import json
import shutil
import tempfile
from pathlib import Path

import red_3738_output_formats as engine

ENVIRONMENTS = ("production", "staging")


def cases(*labels):
    return {
        f"test_real_command_tree[{label}-{environment}]"
        for label in labels for environment in ENVIRONMENTS
    }


def run_reverts():
    originals = {path: (engine.ROOT / path).read_bytes() for _, path, *_ in engine.REVERTS}
    with tempfile.TemporaryDirectory(prefix="red-3849-cli-") as directory:
        repo = Path(directory) / "repo"
        shutil.copytree(engine.ROOT, repo, ignore=shutil.ignore_patterns(
            ".git", ".venv*", ".worktrees", ".claude", "__pycache__", ".pytest_cache", "bin"
        ))
        result, failed, universe, errors = engine.run(repo, Path(directory) / "green.xml", engine.TEST)
        assert result.returncode == 0 and not failed and not errors, result.stdout + result.stderr
        assert engine.CONTROLS <= universe
        print(f"GREEN: {len(universe)} identities, {len(engine.CONTROLS)} controls", flush=True)
        for name, path, before, after, expected in engine.REVERTS:
            source = originals[path].decode()
            assert source.count(before) == 1, name
            (repo / path).write_text(source.replace(before, after, 1))
            try:
                result, failed, passed, errors = engine.run(repo, Path(directory) / f"{name}.xml", engine.TEST)
                expected = {engine.PREFIX + case for case in expected}
                assert result.returncode == 1 and not errors, result.stdout + result.stderr
                assert failed == expected, (name, failed, expected, result.stdout)
                assert passed == universe - expected and engine.CONTROLS <= passed
                print(json.dumps({"revert": name, "pytest_exit": 1, "failed": sorted(failed),
                                  "controls": sorted(engine.CONTROLS)}), flush=True)
            finally:
                (repo / path).write_bytes(originals[path])
    assert all(hashlib.sha256((engine.ROOT / path).read_bytes()).digest() == hashlib.sha256(data).digest()
               for path, data in originals.items())
    print(f"PASS: {len(engine.REVERTS)} exact named reverts; original worktree unchanged")


if __name__ == "__main__":
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_sessions_3849.py"
    engine.PREFIX = "scripts.red.test_sessions_3849::"
    engine.CONTROLS = {
        engine.PREFIX + case for case in cases(
            "fallback_flag", "fallback_environment", "refresh_success", "valid_oauth",
            "no_session", "whoami_valid", "whoami_with_refresh",
        )
    }
    lost_reason = cases("refresh_error", "cause_bad_json", "cause_network")
    engine.REVERTS = [
        (
            "discard_refresh_error", "cmd/root.go",
            "refreshErr = err", "_ = err", lost_reason,
        ),
        (
            "erase_original_refresh_reason", "cmd/root.go",
            "refreshErr = err", 'refreshErr = fmt.Errorf("refresh refused")', lost_reason,
        ),
        (
            "reject_config_fallback_after_refresh_failure", "cmd/root.go",
            'if apiKey := config.GetAPIKey(); apiKey != "" {',
            'if apiKey := config.GetAPIKey(); apiKey != "" && refreshErr == nil {',
            cases("fallback_config"),
        ),
        (
            "promise_renewal_without_refresh_token", "cmd/auth.go",
            'if config.GetRefreshToken(env) != "" {', 'if true {',
            cases("whoami_no_refresh"),
        ),
        (
            "restore_inner_production_login_advice", "internal/auth/token.go",
            'fmt.Errorf("token refresh failed (status %d)", resp.StatusCode)',
            'fmt.Errorf("token refresh failed (status %d) — please run \'stompy login\' again", resp.StatusCode)',
            cases("refresh_error"),
        ),
    ]
    run_reverts()
