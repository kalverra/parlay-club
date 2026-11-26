# Final fix

## Commit

Mount push secrets and re-arm dev reminders

The deploy workflow now mounts the VAPID pair and reminder token from Secret Manager and passes `REMINDER_QUEUE` from a
repository variable, so production never keeps the private key or bearer token in plain revision config. The dev timer
scheduler also reports itself as non-durable, so `Sync` re-arms lost timers after a restart instead of trusting a
`reminder_tasks` row for a process that no longer exists.
