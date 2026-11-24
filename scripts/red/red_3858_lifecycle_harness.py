"""Five behavioral cuts with exact named failures and unchanged live controls."""

from pathlib import Path

import red_3849_sessions as driver


if __name__ == "__main__":
    engine = driver.engine
    engine.ROOT = Path(__file__).resolve().parents[2]
    engine.TEST = "scripts/red/test_lifecycle_harness_3858.py"
    engine.PREFIX = "scripts.red.test_lifecycle_harness_3858::"
    case = lambda label: "test_real_harness[" + label + "]"
    engine.CONTROLS = {
        engine.PREFIX + case("control_lifecycle_readback"),
        engine.PREFIX + case("control_command_registration"),
    }
    flags = {case(label) for label in (
        "top_level_flags", "nested_flags", "deep_local_and_persistent", "slice_flags"
    )}
    path = "cmd/lifecycle_harness_helpers_test.go"
    engine.REVERTS = [
        (
            "wait_forever_for_a_worker_that_never_started", path,
            "timeout := time.After(100 * time.Millisecond)",
            "timeout := make(chan time.Time)",
            {case(label) for label in ("unknown_flag", "invalid_flag_value", "early_pre_run_error")},
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
