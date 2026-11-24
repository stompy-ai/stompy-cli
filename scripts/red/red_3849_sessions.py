"""Four semantic decisions, exact pytest exit 1 and named invariant controls."""

from pathlib import Path

import red_3738_output_formats as engine

ENVIRONMENTS = ("production", "staging")


def cases(*labels):
    return {
        f"test_real_command_tree[{label}-{environment}]"
        for label in labels for environment in ENVIRONMENTS
    }


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
    ]
    engine.main()
