"""Five behavioral cuts with exact named failures and unchanged live controls."""

from pathlib import Path

import red_3849_sessions as driver


if __name__ == "__main__":
    engine = driver.engine
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_cli_harness_3859.py"
    engine.PREFIX = "scripts.red.test_cli_harness_3859::"
    case = lambda label: "test_real_harness[" + label + "]"
    engine.CONTROLS = {
        engine.PREFIX + case("control_search_json"),
        engine.PREFIX + case("control_server_error"),
    }
    flags = {case(label) for label in (
        "omitted_context_list", "deep_local_and_persistent", "slice_flags"
    )}
    path = "cmd/lifecycle_harness_helpers_test.go"
    engine.REVERTS = [
        (
            "wait_forever_for_a_worker_that_never_started", path,
            "timeout := time.After(100 * time.Millisecond)",
            "timeout := make(chan time.Time)",
            {case(label) for label in ("unknown_flag",)},
        ),
        (
            "skip_descendant_flag_resets", path,
            "resetLifecycleFlags(t, child)",
            "if false { resetLifecycleFlags(t, child) }",
            flags,
        ),
        (
            "retain_scalar_flag_values", path,
            "flag.Value.Set(flag.DefValue)",
            "flag.Value.Set(flag.Value.String())",
            flags - {case("slice_flags")},
        ),
        (
            "retain_changed_bits", path,
            "flag.Changed = false", "_ = flag.Changed", flags,
        ),
        (
            "retain_slice_contents", path,
            "slice.Replace(nil)", "slice.Replace(slice.GetSlice())",
            {case("slice_flags")},
        ),
    ]
    driver.run_reverts()
