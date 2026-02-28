# Spike 005: Cobra Alias Behavior — Findings

**Status:** DONE
**Date:** 2026-02-28
**Cobra version:** v1.10.2

---

## Summary

All bilingual alias requirements for दूतसभा are achievable with Cobra v1.10.2.
Five patterns are confirmed; one gotcha requires a workaround.

---

## Finding 1: `cobra.ArbitraryArgs` is REQUIRED for extension discovery

**Critical.** Without it, cobra returns `"unknown command"` error *before*
calling `root.RunE`, even when `RunE` is defined on the root command.

```go
// WRONG: root.RunE is never called for unknown commands
root := &cobra.Command{Use: "dootsabha", RunE: extensionHandler}

// CORRECT: ArbitraryArgs makes cobra pass unmatched args to root.RunE
root := &cobra.Command{
    Use:  "dootsabha",
    Args: cobra.ArbitraryArgs,   // ← REQUIRED
    RunE: func(cmd *cobra.Command, args []string) error {
        if len(args) == 0 { return cmd.Help() }
        return extensionDiscovery(cmd, args[0])
    },
}
```

**Behaviour confirmed:**
- `dootsabha unknown-cmd` → root.RunE called with `args=["unknown-cmd"]` ✓
- `dootsabha council "prompt"` → council.RunE called (subcommands route first) ✓
- `dootsabha` (no args) → root.RunE called with `args=[]` → show help ✓

---

## Finding 2: `cmd.CalledAs()` tracks which alias was used

Cobra's `cmd.CalledAs()` returns the exact string the user typed (primary name
or alias). Use this for logging and for building bilingual output.

```
$ dootsabha council "test"   →  CalledAs() = "council"
$ dootsabha sabha   "test"   →  CalledAs() = "sabha"
$ dootsabha sab     "test"   →  CalledAs() = "sabha"   (prefix-matched alias)
```

**Production use:** Log the called-as name in session headers.

---

## Finding 3: Help text rendering — Aliases block, not inline

Cobra renders aliases in a dedicated `Aliases:` block on the subcommand's own
`--help`, but the *parent* help table shows only the primary name.

**Parent help (`dootsabha --help`):**
```
Available Commands:
  consult     consult (paraamarsh) — Query a single agent
  council     council (sabha) — Run multi-agent council deliberation
  status      status (sthiti) — Show agent health and config
```
→ Sanskrit alias visible only via `Short` field phrasing (e.g., `"council (sabha)"`).

**Subcommand help (`dootsabha council --help`):**
```
Aliases:
  council, sabha
```
→ Full alias list rendered automatically from `Aliases []string`.

**Production pattern:** Set `Short = "council (sabha) — ..."` so the alias
is surfaced in the parent table, and add `Aliases: []string{"sabha"}` for
the dedicated help block and routing.

---

## Finding 4: Devanagari (non-ASCII) aliases work fully

Cobra dispatches by Go string equality — no ASCII restriction.

```go
Aliases: []string{"sabha", "सभा"}
```

- `dootsabha सभा "prompt"` → routes to council ✓
- `--help` shows `Aliases: council, sabha, सभा` ✓
- Terminal rendering depends on font/locale; bytes are passed through faithfully.

**Recommendation:** Include Devanagari aliases for completeness but keep
romanised aliases (sabha, paraamarsh) as the primary aliases for CLI usability
on systems that may not support Devanagari input.

---

## Finding 5: Prefix matching includes aliases

With `cobra.EnablePrefixMatching = true`:

| Input | Routes to | CalledAs |
|-------|-----------|----------|
| `coun` | council | "council" |
| `sab`  | council (via alias) | "sabha" |
| `par`  | consult (via alias) | "paraamarsh" |
| `sthi` | status (via alias) | "sthiti" |
| `s`    | **ambiguous** → extension discovery | — |

**Ambiguity rule:** If the prefix matches multiple commands/aliases, cobra
does NOT route to any of them — it falls to root.RunE (extension discovery).
For `s`, the suggestion engine only checks primary names, so it suggests
`status` (not `sabha` or `sthiti`).

**Recommendation:** Enable `cobra.EnablePrefixMatching = true` in production.
It is very ergonomic for both English and Sanskrit aliases.

---

## Finding 6: Flag aliases — pflag has no first-class support

pflag (`github.com/spf13/pflag`) does NOT support two long names for one flag.
The canonical workaround is a **hidden alias flag** resolved in `RunE`:

```go
// In newCouncilCmd():
f.StringVarP(&agent, "agent", "a", "", "Agent to use")
f.String("doota", "", "Alias for --agent (दूत)")
must(f.MarkHidden("doota"))   // hides from --help

// In RunE:
doota, _ := cmd.Flags().GetString("doota")
if doota != "" && agent == "" {
    agent = doota
}
```

**Confirmed working:**
```
$ dootsabha council --agent claude "test"   → agent="claude" ✓
$ dootsabha council --doota  codex "test"   → agent="codex"  ✓
$ dootsabha council --kaalseema 2m "test"   → timeout="2m"   ✓
```

**Limitation:** `--doota` appears in `--help` as `[hidden]` unless hidden.
Hidden flags are excluded from help text but still parsed correctly.

---

## Finding 7: Tab completion does NOT include aliases by default

`go run main.go __complete ""` returns only primary command names.
`sab<TAB>` produces no completion candidates.

**Workaround:** Add a `ValidArgsFunction` on the root command that injects
alias completions with their parent command's description:

```go
root.ValidArgsFunction = func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
    var completions []string
    for _, sub := range cmd.Commands() {
        for _, alias := range sub.Aliases {
            if strings.HasPrefix(alias, toComplete) {
                completions = append(completions, alias+"\t"+sub.Short)
            }
        }
    }
    return completions, cobra.ShellCompDirectiveNoFileComp
}
```

Result: `sab<TAB>` → `sabha   council (sabha) — ...` ✓

---

## Finding 8: `SuggestionsFor` does NOT search aliases

`cmd.SuggestionsFor("sabh")` returns `[]` (empty) — only primary names are
checked. This affects the "Did you mean X?" message for typos.

**Impact:** A user typing `dootsabha saaba` gets no suggestion, even though
`sabha` is a registered alias. For production, either accept this limitation
or override the unknown-command handler to also call `SuggestionsFor` on the
alias set.

---

## Production Recommendations

| Area | Pattern |
|------|---------|
| Extension discovery | `Args: cobra.ArbitraryArgs` on root **+ explicit `RunE`** |
| Bilingual naming | `Use = "council"`, `Short = "council (sabha) — ..."`, `Aliases = []string{"sabha"}` |
| Devanagari | Include in Aliases but keep romanised alias as primary |
| Flag aliases | Hidden alias flag + RunE resolution |
| Tab completion | `ValidArgsFunction` on root to inject alias completions |
| Prefix matching | Enable `cobra.EnablePrefixMatching = true` globally |
| CalledAs logging | Log `cmd.CalledAs()` in session headers for bilingual UX |
| Suggestion typos | `SuggestionsFor` is alias-blind; acceptable limitation |

---

## Verification Log

```
go run main.go --help                     ✓ bilingual names in Available Commands
go run main.go council --help             ✓ Aliases: council, sabha
go run main.go council "test"             ✓ CalledAs="council"
go run main.go sabha "test"               ✓ CalledAs="sabha"
go run main.go paraamarsh "test"          ✓ CalledAs="paraamarsh"
go run main.go sthiti                     ✓ CalledAs="sthiti"
go run main.go unknown-cmd                ✓ extension discovery triggered
go run main.go council --agent claude     ✓ agent="claude"
go run main.go council --doota codex      ✓ agent="codex" (flag alias)
go run main.go council --kaalseema 2m     ✓ timeout="2m" (flag alias)
go run main.go coun "test"                ✓ prefix match → council
go run main.go sab "test"                 ✓ prefix match on alias → sabha
go run main.go s "test"                   ✓ ambiguous → extension discovery
सभा invocation (Devanagari alias)         ✓ routes to council
Aliases block in --help                   ✓ shows Unicode correctly
ValidArgsFunction alias completions       ✓ sab<TAB> works with workaround
```
