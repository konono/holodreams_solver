#!/usr/bin/env python3
"""Generate data/charts.json from SUS files extracted by holodori-asset-tools.

Usage:
    python scripts/generate_charts.py <sus_directory> [--output data/charts.json]

Requires:
    pip install sonolus-level-converters
"""

import argparse
import json
import os
import re
import sys
from glob import glob
from pathlib import Path

from sonolus_converters.holodori_sus import load
from sonolus_converters.notes import Single, Slide, Bpm
from sonolus_converters.notes.holodorievents import HolodoriSkill


# From LiveNote.json (HolodoriDB): flick notes have scoreCoefficientPermilMultiply = 1050
NOTE_WEIGHTS = {
    "tap": 1000,
    "flick": 1050,
    "slide_start": 1000,
    "slide_end": 1000,
    "slide_flick_end": 1000,
    "slide_relay": 100,
    "slide_continue": 100,
}


def beat_to_time_multi_bpm(beat, bpm_changes):
    """Convert beat to time in seconds, handling BPM changes."""
    time = 0.0
    prev_beat = 0.0
    prev_bpm = bpm_changes[0][1]

    for change_beat, change_bpm in bpm_changes[1:]:
        if beat <= change_beat:
            break
        time += (change_beat - prev_beat) * 60.0 / prev_bpm
        prev_beat = change_beat
        prev_bpm = change_bpm

    time += (beat - prev_beat) * 60.0 / prev_bpm
    return time


def parse_sus(filepath):
    """Parse a SUS file and return chart data."""
    with open(filepath) as f:
        score = load(f)

    # Extract BPM changes
    bpm_changes = sorted(
        [(n.beat, n.bpm) for n in score.notes if isinstance(n, Bpm)],
        key=lambda x: x[0],
    )
    if not bpm_changes:
        return None

    def to_time(beat):
        return beat_to_time_multi_bpm(beat, bpm_changes)

    # SP skill points (5 slots)
    skills = sorted(
        [n for n in score.notes if isinstance(n, HolodoriSkill)],
        key=lambda n: n.beat,
    )
    sp_points = [to_time(s.beat) for s in skills]

    # Scoring events
    events = []
    combo = 0

    # Singles (tap / flick)
    for n in score.notes:
        if isinstance(n, Single) and not n.fake:
            is_flick = hasattr(n, "direction") and n.direction != 0
            weight = NOTE_WEIGHTS["flick"] if is_flick else NOTE_WEIGHTS["tap"]
            events.append((n.beat, weight, "flick" if is_flick else "tap"))

    # Slides
    for n in score.notes:
        if isinstance(n, Slide) and not n.fake:
            for i, c in enumerate(n.connections):
                if c.type == "start":
                    weight = NOTE_WEIGHTS["slide_start"]
                    note_type = "slide_start"
                elif c.type == "end":
                    is_flick = hasattr(c, "judgeType") and c.judgeType == "flick"
                    if is_flick:
                        weight = NOTE_WEIGHTS["slide_flick_end"]
                        note_type = "slide_flick_end"
                    else:
                        weight = NOTE_WEIGHTS["slide_end"]
                        note_type = "slide_end"
                elif c.type == "relay":
                    weight = NOTE_WEIGHTS["slide_relay"]
                    note_type = "slide_relay"
                elif c.type == "tick":
                    weight = NOTE_WEIGHTS["slide_continue"]
                    note_type = "slide_continue"
                else:
                    continue
                events.append((c.beat, weight, note_type))

    events.sort(key=lambda x: x[0])

    # Assign combo indices
    notes_out = []
    for beat, weight, note_type in events:
        combo += 1
        notes_out.append({
            "time": round(to_time(beat), 4),
            "weight": weight,
            "combo_index": combo,
        })

    return {
        "special_points": [round(t, 4) for t in sp_points],
        "notes": notes_out,
        "bpm": bpm_changes[0][1],
        "total_notes": len(notes_out),
    }


def main():
    parser = argparse.ArgumentParser(description="Generate charts.json from SUS files")
    parser.add_argument("sus_dir", help="Directory containing SUS files")
    parser.add_argument("--output", default="data/charts.json", help="Output path")
    args = parser.parse_args()

    sus_files = sorted(glob(os.path.join(args.sus_dir, "chart_*.sus")))
    if not sus_files:
        print(f"No SUS files found in {args.sus_dir}", file=sys.stderr)
        sys.exit(1)

    print(f"Processing {len(sus_files)} SUS files...")

    charts = {}
    errors = []

    for filepath in sus_files:
        basename = os.path.basename(filepath)
        match = re.match(r"chart_(m\d{4})_(\w+)\.sus", basename)
        if not match:
            continue

        music_id, difficulty = match.groups()

        try:
            chart = parse_sus(filepath)
            if chart is None:
                errors.append(f"{basename}: no BPM data")
                continue

            key = f"{music_id}_{difficulty}"
            charts[key] = {
                "music_id": music_id,
                "difficulty": difficulty,
                **chart,
            }
        except Exception as e:
            errors.append(f"{basename}: {e}")

    # Load songs.json for duration info
    songs_path = Path(args.output).parent / "songs.json"
    if songs_path.exists():
        with open(songs_path) as f:
            songs_data = json.load(f)
        songs_list = songs_data.get("songs", songs_data) if isinstance(songs_data, dict) else songs_data
        song_durations = {s["id"]: s["playing_seconds"] for s in songs_list}
        for key, chart in charts.items():
            duration = song_durations.get(chart["music_id"])
            if duration:
                chart["duration"] = duration

    output_path = Path(args.output)
    output_path.parent.mkdir(parents=True, exist_ok=True)
    with open(output_path, "w") as f:
        json.dump(charts, f, separators=(",", ":"))

    print(f"Generated {len(charts)} charts → {output_path}")
    if errors:
        print(f"\n{len(errors)} errors:")
        for e in errors[:10]:
            print(f"  {e}")
        if len(errors) > 10:
            print(f"  ... and {len(errors) - 10} more")


if __name__ == "__main__":
    main()
