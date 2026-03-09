# Lessons Learned

- When modifying files, use the dedicated `apply_patch` tool when available instead of invoking `apply_patch` through `exec_command`.
- When the user gives a direct follow-up command like "Do it", switch immediately from advisory discussion to execution and deliver the implementation without additional prompting.
- When benchmark statistics are meant to describe completed work, exclude shutdown-induced `context canceled` or `context deadline exceeded` results from aggregate error counts.
