# Example: Review & Refine Walkthrough

A complete walkthrough of using दूतसभा's review and refine pipelines to
iteratively improve code output.

## Scenario A: Code Review

You have a function and want one agent to produce it and another to review it.

### Step 1: Run Review

```bash
dootsabha review --json \
  "Write a Go function that retries HTTP requests with exponential backoff,
   jitter, and configurable max retries. Include context cancellation support."
```

Default pipeline: codex authors, claude reviews.

### Step 2: Extract Author Output

```bash
jq -r '.author.content' result.json
```

### Step 3: Extract Review Feedback

```bash
jq -r '.review.content' result.json
```

### Step 4: Override Author/Reviewer

```bash
# Gemini authors, claude reviews
dootsabha review --json --author gemini --reviewer claude "Your prompt"

# Claude authors, codex reviews
dootsabha review --json --author claude --reviewer codex "Your prompt"
```

### Step 5: Check Cost

```bash
jq '{
  author_cost: .author.cost_usd,
  review_cost: .review.cost_usd,
  total: .meta.total_cost_usd
}' result.json
```

---

## Scenario B: Iterative Refinement

You want an author to produce content, then have multiple reviewers
critique it sequentially with the author incorporating each round of feedback.

### Step 1: Run Refine

```bash
dootsabha refine --json \
  "Write a production-ready rate limiter middleware for a Go HTTP server.
   Support per-IP and per-API-key limits with sliding window algorithm."
```

Default pipeline: claude authors, codex and gemini review in order.

### Step 2: See the Evolution

```bash
# How many versions were produced
jq '.versions | length' result.json

# See each version's reviewer and what changed
jq -r '.versions[] | "v\(.version) [\(.reviewer // "initial")]:\n\(.content)\n---\n"' result.json
```

### Step 3: Get the Final Version

```bash
jq -r '.final.content' result.json
```

### Step 4: Read Review Feedback

```bash
# See what each reviewer said
jq -r '.versions[] | select(.review) | "--- \(.reviewer) ---\n\(.review)\n"' result.json
```

### Step 5: Custom Author and Reviewer Order

```bash
# Codex authors, reviewed by claude then gemini
dootsabha refine --json --author codex --reviewers claude,gemini "Your prompt"

# Single reviewer
dootsabha refine --json --author claude --reviewers codex "Your prompt"
```

### Step 6: Anonymous Mode

By default, reviewer identities are anonymized in prompts sent to the author
(the author sees "a reviewer" instead of "codex"). Disable this:

```bash
dootsabha refine --json --anonymous=false "Your prompt"
```

### Step 7: Cost Breakdown

```bash
jq '{
  total_cost: .meta.total_cost_usd,
  total_duration_s: (.meta.duration_ms / 1000),
  per_version: [.versions[] | {version, provider, cost_usd, reviewer}]
}' result.json
```

---

## Combining Review + Refine in a Workflow

For maximum quality, use review for a quick check, then refine if needed:

```bash
# Quick review first
dootsabha review --json "Write a connection pool manager" > review.json
REVIEW=$(jq -r '.review.content' review.json)

# If review found issues, run full refinement
if echo "$REVIEW" | grep -qi "issue\|bug\|problem\|concern"; then
  echo "Issues found — running refinement pipeline"
  dootsabha refine --json "Write a connection pool manager" > refined.json
  jq -r '.final.content' refined.json
else
  echo "Review passed — using author output"
  jq -r '.author.content' review.json
fi
```
