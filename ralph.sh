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

PRETTY_STREAM=${PRETTY_STREAM:-true}

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

pretty_stream_json() {
    local line type block_type delta_text delta_type partial
    local in_text=false
    local in_tool=false
    local tool_name=""
    local tool_id=""
    local tool_input=""
    local printed=false

    while IFS= read -r line; do
        [ -z "$line" ] && continue

        type=$(printf '%s' "$line" | jq -r '.type // empty' 2>/dev/null || true)

        case "$type" in
        assistant)
            local item item_type item_text item_name item_id item_input
            while IFS= read -r item; do
                item_type=$(printf '%s' "$item" | jq -r '.type // empty' 2>/dev/null || true)
                case "$item_type" in
                text)
                    item_text=$(printf '%s' "$item" | jq -r '.text // empty' 2>/dev/null || true)
                    [ -n "$item_text" ] && printf "assistant: %s\n" "$item_text"
                    ;;
                tool_use)
                    item_name=$(printf '%s' "$item" | jq -r '.name // "tool"' 2>/dev/null || true)
                    item_id=$(printf '%s' "$item" | jq -r '.id // empty' 2>/dev/null || true)
                    item_input=$(printf '%s' "$item" | jq -c '.input // empty' 2>/dev/null || true)
                    if [ -n "$item_id" ]; then
                        printf "tool: %s (%s)\n" "$item_name" "$item_id"
                    else
                        printf "tool: %s\n" "$item_name"
                    fi
                    if [ -n "$item_input" ] && [ "$item_input" != "null" ]; then
                        printf "tool_input: %s\n" "$item_input"
                    fi
                    ;;
                *) ;;
                esac
            done < <(printf '%s' "$line" | jq -c '.message.content[]?' 2>/dev/null || true)
            ;;
        user)
            local item item_type item_text item_content
            while IFS= read -r item; do
                item_type=$(printf '%s' "$item" | jq -r '.type // empty' 2>/dev/null || true)
                case "$item_type" in
                text)
                    item_text=$(printf '%s' "$item" | jq -r '.text // empty' 2>/dev/null || true)
                    [ -n "$item_text" ] && printf "user: %s\n" "$item_text"
                    ;;
                tool_result)
                    item_content=$(printf '%s' "$item" | jq -r '.content // empty' 2>/dev/null || true)
                    [ -n "$item_content" ] && printf "tool_result: %s\n" "$item_content"
                    ;;
                *) ;;
                esac
            done < <(printf '%s' "$line" | jq -c '.message.content[]?' 2>/dev/null || true)
            ;;
        content_block_start)
            block_type=$(printf '%s' "$line" | jq -r '.content_block.type // empty' 2>/dev/null || true)
            if [ "$block_type" = "text" ]; then
                if [ "$in_text" = false ]; then
                    [ "$printed" = true ] && printf "\n"
                    printf "assistant: "
                    in_text=true
                    printed=true
                fi
            elif [ "$block_type" = "tool_use" ]; then
                [ "$in_text" = true ] && printf "\n"
                in_text=false

                tool_name=$(printf '%s' "$line" | jq -r '.content_block.name // "tool"' 2>/dev/null || true)
                tool_id=$(printf '%s' "$line" | jq -r '.content_block.id // empty' 2>/dev/null || true)
                tool_input=""
                in_tool=true
                printed=true

                if [ -n "$tool_id" ]; then
                    printf "tool: %s (%s)\n" "$tool_name" "$tool_id"
                else
                    printf "tool: %s\n" "$tool_name"
                fi
            fi
            ;;
        content_block_delta)
            delta_text=$(printf '%s' "$line" | jq -r '.delta.text // empty' 2>/dev/null || true)
            if [ -n "$delta_text" ]; then
                if [ "$in_text" = false ]; then
                    [ "$printed" = true ] && printf "\n"
                    printf "assistant: "
                    in_text=true
                    printed=true
                fi
                printf "%s" "$delta_text"
                continue
            fi

            delta_type=$(printf '%s' "$line" | jq -r '.delta.type // empty' 2>/dev/null || true)
            if [ "$delta_type" = "input_json_delta" ]; then
                partial=$(printf '%s' "$line" | jq -r '.delta.partial_json // empty' 2>/dev/null || true)
                tool_input="${tool_input}${partial}"
            fi
            ;;
        content_block_stop)
            if [ "$in_text" = true ]; then
                printf "\n"
                in_text=false
            fi
            if [ "$in_tool" = true ]; then
                if [ -n "$tool_input" ]; then
                    if jq -e . >/dev/null 2>&1 <<<"$tool_input"; then
                        printf "tool_input: %s\n" "$(jq -c . <<<"$tool_input" 2>/dev/null || echo "$tool_input")"
                    else
                        printf "tool_input: %s\n" "$tool_input"
                    fi
                fi
                in_tool=false
            fi
            ;;
        message_stop)
            [ "$in_text" = true ] && printf "\n"
            in_text=false
            ;;
        error)
            local err
            err=$(printf '%s' "$line" | jq -r '.error.message // .message // .' 2>/dev/null || true)
            [ "$in_text" = true ] && printf "\n"
            in_text=false
            printf "error: %s\n" "$err"
            ;;
        *) ;;
        esac
    done

    if [ "$in_text" = true ]; then
        printf "\n"
    fi
    return 0
}

while true; do
    if [ $MAX_ITERATIONS -gt 0 ] && [ $ITERATION -ge $MAX_ITERATIONS ]; then
        echo "Reached max iterations: $MAX_ITERATIONS"
        break
    fi

    # Run Ralph iteration with selected prompt
    # -p: Headless mode (non-interactive, reads from stdin)
    # --dangerously-skip-permissions: Auto-approve all tool calls (YOLO mode)
    # --output-format=stream-json: Structured output for logging/monitoring
    # --model opus: Primary agent uses Opus for complex reasoning (task selection, prioritization)
    #               Can use 'sonnet' in build mode for speed if plan is clear and tasks well-defined
    # --verbose: Detailed execution logging
    if [ "$PRETTY_STREAM" = true ] && command -v jq >/dev/null 2>&1; then
        cat "$PROMPT_FILE" | claude-glm -p \
            --dangerously-skip-permissions \
            --output-format=stream-json \
            --model opus \
            --verbose | pretty_stream_json
    else
        if [ "$PRETTY_STREAM" = true ] && ! command -v jq >/dev/null 2>&1; then
            echo "Note: jq not found; falling back to raw stream-json output." >&2
        fi
        cat "$PROMPT_FILE" | claude-glm -p \
            --dangerously-skip-permissions \
            --output-format=stream-json \
            --model opus \
            --verbose
    fi

    # Push changes after each iteration
    git push origin "$CURRENT_BRANCH" || {
        echo "Failed to push. Creating remote branch..."
        git push -u origin "$CURRENT_BRANCH"
    }

    ITERATION=$((ITERATION + 1))
    echo -e "\n\n======================== LOOP $ITERATION ========================\n"
done
