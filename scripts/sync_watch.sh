#!/bin/sh

REMOTE="clawbot@10.90.0.197:~/projects/openclaw-dashboard"
WATCH_EXTS="\.go$|\.html$|\.tmpl$|\.nix$|\.toml$|\.css$|\.js$"
TRIGGER_FILE=".sync_trigger"

# Cleanup function
cleanup() {
    echo "Terminating sync..."
    rm -f "$TRIGGER_FILE"
    # Kill all child processes (background watcher)
    pkill -P $$
    exit
}
trap cleanup INT TERM

echo "🚀 Starting Ultra-Fast Debounced Sync..."

# 1. Initial sync
echo "🔄 Initial sync..."
rsync -avz --delete --exclude '.git/' --exclude 'tmp/' ./ "$REMOTE"

# 2. Watcher (Background)
# We handle filtering inside the loop to avoid pipe buffering issues with grep
fswatch -r -l 0.2 \
    --event Created --event Updated --event Removed --event Renamed \
    . 2>/dev/null | while read -r changed_path; do
    
    # Check if the change is in .git, tmp, openclaw, or is the trigger file
    if echo "$changed_path" | grep -vE "\.git|tmp/|openclaw$|\.sync_trigger" > /dev/null; then
        echo "📝 File changed: $changed_path"
        touch "$TRIGGER_FILE"
    fi
done &

echo "👀 Watching for changes..."

# 3. Sync Loop (Foreground)
while true; do
    if [ -f "$TRIGGER_FILE" ]; then
        # Debounce: wait 0.5s to let multiple file saves accumulate
        echo "⚡ Change detected, waiting for burst to settle..."
        sleep 0.5
        
        # Remove trigger file effectively "claiming" the events
        rm -f "$TRIGGER_FILE"
        
        echo "🔄 Syncing..."
        if rsync -rtvz --delete \
          --exclude '.git/' \
          --exclude 'tmp/' \
          --exclude 'openclaw' \
          --exclude 'server' \
          ./ "$REMOTE"; then
            echo "✅ Sync completed at $(date '+%H:%M:%S')"
        else
            echo "❌ Sync failed!"
        fi
    else
        # Sleep slightly to prevent high CPU usage in the check loop
        sleep 0.2
    fi
done
