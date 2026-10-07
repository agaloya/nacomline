# Nacomline

A manager for ORCA quantum-chemistry jobs on **one computer**: drop inputs in a folder and
it runs them in the background while you work or sleep. It respects your schedule, battery
and CPU temperature, resumes after power cuts, and sorts results into folders. Outputs that
look wrong (ORCA errors, unconverged optimizations, inconsistent geometries) are set apart.

**The family.**
Three related projects by the same author, all under the same license:

- [Matriline](https://github.com/agaloya/matriline): ORCA jobs on many computers, with
  security and verification of results from machines you do not control.
- [Nacomline](https://github.com/agaloya/nacomline): ORCA jobs on one computer.
- [Catenaline](https://github.com/agaloya/catenaline): a proof of concept for any program
  that turns input files into output files, and for chains of programs.

Status: first working version (Linux, Windows and macOS). Nacomline does not copy
Matriline: it runs Matriline's own server and client on this computer only (127.0.0.1),
generated from one settings file, so every Matriline improvement reaches it without a new
Nacomline.

## Install and use

1. Put `nacomline`, `matriline-server` and `matriline-client` in one folder, e.g. `~/bin`.
   Build `nacomline` with `go build` in this folder (Go 1.27 or later); build Matriline's
   two programs from its source as its
   [INSTALL.md](https://github.com/agaloya/matriline/blob/main/INSTALL.md) explains, or
   download them from its releases page. ORCA must be installed (Nacomline finds it).
2. `nacomline init ~/my-project` creates the project folder with `nacomline.conf`, the only
   file you edit (ORCA folder, cores and memory to use, schedule, battery, temperature,
   alerts).
3. Put inputs (`*.inp`, sub-folders allowed) in `~/my-project/input/` and start it:
   `nacomline -c ~/my-project/nacomline.conf run`, or once
   `nacomline -c ~/my-project/nacomline.conf service install` to start it by itself.
4. Results arrive in `output/`; ORCA failures in `errors/`; outputs that look wrong
   (unconverged optimizations, inconsistent geometries) in `weird/` with the reason
   (`nacomline review`).

Following it: `nacomline status`, `nacomline watch` (live view: input, a drawing of the
molecule, the latest output lines), `nacomline web` (a local web page with the same and a
settings page), `nacomline console` (menus). `nacomline pause 2h` stops using the computer
for a while, `nacomline resume` ends it. `nacomline help` lists the rest.

## How it works

`nacomline init` runs Matriline's `init` for the project and keeps Matriline's own
configuration in the hidden `.nacomline/` folder: the server listens on 127.0.0.1 only,
and everything that only matters between several computers (keys, bans, quarantine,
second opinions, re-checks on other hosts) is off. ORCA's output checks stay on.
`nacomline run` starts the server and the client, restarts either if it ends, and restarts
the client when its settings change. Changing settings: edit `nacomline.conf` and run
`nacomline config apply`, or use the web page's settings.

License: AGPL-3.0 with an author-attribution additional term (see [docs/NOTICE](docs/NOTICE)).
Citation: see [CITATION.cff](CITATION.cff).

## Why "Nacomline"

Named after Gonzalo Guerrero, the Spanish castaway who became a *nacom*, a war captain, in
Yucatán: one person carrying the whole campaign alone. The ending *-line* follows its
sibling Matriline.
