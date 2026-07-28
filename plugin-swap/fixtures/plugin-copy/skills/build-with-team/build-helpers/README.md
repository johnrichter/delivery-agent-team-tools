# plugin-swap fixture: embedded build-helpers copy

Stand-in for the real plugin's embedded `build-helpers` tree (Go source +
vendor + prebuilt `.bin/`) -- a `go.mod`/`main.go` stub, not the full vendored
source, since `swap-plugin.sh`'s only operation on this directory is to
delete it (SC-DAT-FROZEN retires the embedded copy). `verify.sh` places a
real built binary under `.bin/` at run time, into a disposable copy of this
fixture, before exercising the pre-swap ("BEFORE") harness path.
