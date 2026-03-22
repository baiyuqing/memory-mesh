# Lessons Learned

- When the user wants a clean environment for a new feature branch, create and use a separate `git worktree` instead of repurposing the main checkout.
- Keep implementation work and task-tracking edits inside the dedicated development worktree so `main` stays untouched.
- When the user explicitly authorizes autonomous execution, move straight from research or planning into implementation and verification without asking follow-up questions.
