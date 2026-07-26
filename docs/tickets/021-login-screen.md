---
id: 21
title: Login screen — token entry and auth gate
role: frontend
depends: [8, 9]
status: done
---
The UI has no way to authenticate: `createSession` exists but nothing calls it, so a fresh
browser hits 401 with no recourse. Add a login screen and an auth gate so a user can enter the
FLIGHTDECK_TOKEN, get a session cookie, and use the app — and is shown the login when not
authenticated.

## Acceptance criteria
- [ ] On load, the app probes auth; when unauthenticated (a 401 from an authed endpoint) it
      shows a Login screen instead of the app, and shows the app once authenticated.
- [ ] The Login screen takes the token (`type=password`), calls `createSession`, and on success
      reveals the app; a bad token shows an inline error. The token is never logged or kept in
      state past submit.
- [ ] DESIGN.md tokens only; a real keyboard-accessible form (labelled input, submit on Enter,
      focus-visible), dark, on-brand (Space Grotesk title, accent primary).
- [ ] Behaviour tests against a mocked client: unauth → Login shown; submitting a good token
      calls createSession and reveals the app; a bad token surfaces the error. lint + build +
      test pass.

## Handoff

The app is now usable from a fresh browser. `App.tsx` is an auth gate with three states —
`checking` (brief, bare `bg-bg`), `authed` (the routed app), `unauthed` (the Login screen). On
mount it probes `listProjects()`; **only** an `ApiError` with `status === 401` means unauthed —
any other error falls through to `authed` so a backend hiccup never traps the user on Login.

`pages/Login.tsx` is a centered dark card: `font-display` "FlightDeck" title, one `type=password`
Token field (reusing `RegisterProjectDialog`'s input class), a real `<form onSubmit>` (Enter
submits), an accent "Sign in" button disabled while in flight, an inline `role="alert"` error
("That token was not accepted." on 401, else `ApiError.message`), and a mono hint naming
`FLIGHTDECK_TOKEN`. On success it calls `createSession(token)` then `onAuthenticated()`; the token
is cleared from state on both branches, never logged. Verified end to end in the running demo:
fresh load → Login → enter `FLIGHTDECK_TOKEN` → Fleet.

Not built: sign-out (there is no `DELETE /api/session` endpoint; the session is an httpOnly
cookie that clears server-side on restart / cookie expiry). A future logout would need that
endpoint. DESIGN.md tokens only.
