#!/usr/bin/env bash
# Usage: ./loop.sh [plan] [max_iterations]
# Examples:
#   ./loop.sh              # Build mode, unlimited iterations
#   ./loop.sh 20           # Build mode, max 20 iterations
#   ./loop.sh plan         # Plan mode, unlimited iterations
#   ./loop.sh plan 5       # Plan mode, max 5 iterations

set -euo pipefail

# Parse arguments
if [ "${1:-}" = "plan" ]; then
    # Plan mode
    MODE="plan"
    PROMPT_FILE="PROMPT_plan.md"
    MAX_ITERATIONS=${2:-0}
elif [[ "${1:-}" =~ ^[0-9]+$ ]]; then
    # Build mode with max iterations
    MODE="build"
    PROMPT_FILE="PROMPT_build.md"
    MAX_ITERATIONS=$1
else
    # Build mode, unlimited (no arguments or invalid input)
    MODE="build"
    PROMPT_FILE="PROMPT_build.md"
    MAX_ITERATIONS=0
fi

ITERATION=0
CURRENT_BRANCH=$(git branch --show-current)

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Mode:   $MODE"
echo "Prompt: $PROMPT_FILE"
echo "Branch: $CURRENT_BRANCH"
[ $MAX_ITERATIONS -gt 0 ] && echo "Max:    $MAX_ITERATIONS iterations"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Verify prompt file exists
if [ ! -f "$PROMPT_FILE" ]; then
    echo "Error: $PROMPT_FILE not found"
    exit 1
fi

while true; do
    if [ $MAX_ITERATIONS -gt 0 ] && [ $ITERATION -ge $MAX_ITERATIONS ]; then
        echo "Reached max iterations: $MAX_ITERATIONS"
        break
    fi

    # Run Claude headless with stream-json
    claude-glm -p \
        --dangerously-skip-permissions \
        --output-format stream-json \
        --verbose \
        <"$PROMPT_FILE" | jq -Rr '
            try (fromjson |
                if .type == "assistant" then
                    .message.content[]? |
                    if .type == "tool_use" then
                        if .name == "Task" then
                            "→ Task: \(.input.description // .input.subagent_type // "?")"
                        elif .name == "Read" then
                            "→ Read: \(.input.file_path // "?")"
                        elif .name == "Glob" then
                            "→ Glob: \(.input.pattern // "?")"
                        elif .name == "Grep" then
                            "→ Grep: \(.input.pattern // "?")"
                        elif .name == "Edit" then
                            "→ Edit: \(.input.file_path // "?")"
                        elif .name == "Write" then
                            "→ Write: \(.input.file_path // "?")"
                        elif .name == "Bash" then
                            "→ Bash: \(.input.command // "?" | split("\n")[0])"
                        else
                            "→ \(.name)"
                        end
                    elif .type == "text" then
                        .text
                    else
                        empty
                    end
                else
                    empty
                end
            ) catch empty
        '

    # Push changes after each iteration
    git push origin "$CURRENT_BRANCH" || {
        echo "Failed to push. Creating remote branch..."
        git push -u origin "$CURRENT_BRANCH"
    }

    ITERATION=$((ITERATION + 1))
    echo -e "\n\n======================== LOOP $ITERATION ========================\n"
done
