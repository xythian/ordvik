---
name: name
description: Generate names for projects, tools, services, and concepts by combining stems from real languages into unique, short, evocative compound words. Use when the user needs to name something.
allowed-tools: Read, Grep, Glob, WebSearch, WebFetch, AskUserQuestion
---

# Naming Skill

Generate names by combining stems from real languages into novel compound words. The goal is names that are unique, short (2-3 syllables), evocative of the concept, and sound like they could be real words.

## Method

### Step 1: Understand what's being named

Ask the user (or infer from context):
- What does this thing do?
- What are the key concepts, metaphors, or associations?
- Any naming constraints? (must be available as a GitHub repo, npm package, domain, etc.)
- Any style preferences? (serious vs playful, technical vs natural)

### Step 2: Map the conceptual territory

Identify 4-6 conceptual angles on what's being named. Think in metaphors and associations, not literal descriptions.

Example for a planning/dispatch toolkit:
- Ordering/arranging work (Latin *ordo*, Greek *taxis*)
- Weaving threads together (Old Norse *vefr*, Latin *texere*)
- Inscribing/recording plans (Old Norse *rista*)
- Pathfinding/wayfinding (Latin *trames*, Old English *stig*)
- Craft/method (Old Norse *virk*, Latin *modus*)
- Harbor/gathering place (Old Norse *vik*, Latin *portus*)

### Step 3: Research stems

For each conceptual angle, find stems from multiple language families:

**Productive language families:**
- **Latin** — formal, structured feel. Good for: order, systems, knowledge, law
- **Greek** — technical, scientific feel. Good for: measurement, form, knowledge, process
- **Old Norse** — strong, compact, evocative. Good for: craft, place, action, nature
- **Old English** — grounded, familiar-but-archaic. Good for: counsel, path, knowledge, making
- **Germanic (German/Dutch)** — work-oriented, compound-friendly. Good for: craft, work, building
- **Celtic** — distinctive, less common in tech. Good for: vision, craft, nature
- **Japanese** — clean, minimal. Good for: aesthetic, precision, nature concepts

For each stem, note:
- The word and language of origin
- The meaning
- How it sounds in combination (does it flow, or does it create awkward consonant clusters?)

### Step 4: Combine stems

Form compounds by pairing stems from different concepts (and usually different languages). The best names combine:
- A **concept stem** (what it does or represents)
- A **character stem** (how it feels — place, action, quality)

**Combination patterns that work:**
- concept + place: *ordvik* (order + harbor)
- animal/nature + quality: *leitdar* (search + gift-giver)
- visual + force: *merkvis* (mark + force/sight)
- action + result: *rista* (to carve — stands alone)
- path + craft: *stigwerk* (path + work)

**What to avoid:**
- Existing software project names (search GitHub/npm)
- Combinations that form unfortunate words in common languages
- Names that are hard to pronounce or spell
- Names longer than 3 syllables
- Names that sound too much like common English words

### Step 5: Present candidates

Present 3-4 strong candidates using `AskUserQuestion`. For each:
- The name
- Etymology (language roots and meanings)
- Why it fits (the evocative connection)
- Any practical concerns (availability, pronunciation)

Let the user choose. Don't oversell any option — present them neutrally and let the name resonate (or not).

## Examples

| Project | Name | Etymology | Meaning |
|---------|------|-----------|---------|
| Search service | leitdar | leit (Old Norse: search) + dar (Old Norse: gift/giver) | Search-giver |
| Photo tagging | merkvis | merki (Old Norse: mark) + vis (Latin: force/sight) | Mark-sight |
| Planning toolkit | ordvik | ordo (Latin: order) + vik (Old Norse: harbor/inlet) | Ordered harbor |

## Invocation

The user invokes as `/ordvik:name` or asks conversationally to name something.

**With arguments:** `/ordvik:name <description of what needs naming>`
- Jump straight to Step 2 with the description as input

**Without arguments:** Start with Step 1 questions.

## Recording the name

After the user selects a name, offer to record the etymology in the project's README or a `docs/naming.md` file so the origin isn't lost.
