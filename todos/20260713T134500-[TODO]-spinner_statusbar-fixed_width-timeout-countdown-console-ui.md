# TODO: Move spinners to fixed-width status bar with countdown timeout

## WHAT MUST BE DONE
Move the "Connecting", "Waiting", "Thinking" spinner states from inline output into a dedicated status bar area. The status bar must have a fixed character width so its content changes never shift or disturb the text displayed after it. Each state must show a descending countdown timer (e.g., `|Thinking 4s`). When the countdown reaches 0, a timeout is triggered. Each spinner state has its own independent timeout duration.

## WHY IT MUST BE DONE
Currently the spinner text changes length and position during streaming, which causes the terminal output to jump and shift surrounding content. This disrupts readability and makes the console feel unstable. A fixed-width status bar eliminates visual jitter. The countdown timer gives the user feedback on how long the current state has been active and when it will time out, improving perceived responsiveness and transparency.

## HOW IT MUST BE DONE
- Implement a status bar region in the console transport with a fixed character width (pad shorter states with spaces to match the widest state).
- Display the current state as a single line, e.g., `|Thinking 4s`, `|Waiting 3s`, `|Connecting 5s`.
- Each state (connecting, waiting, thinking) must have its own configurable timeout value in seconds.
- The countdown must decrement every second and display the remaining seconds.
- When the countdown reaches 0, trigger the corresponding timeout behavior (error or abort the current operation).
- The status bar update must use ANSI cursor positioning or equivalent to overwrite only the status bar line, without affecting lines above or below it.
- Remove any existing inline spinner output so all spinner feedback goes exclusively through the status bar.
- The fixed width must be calculated at startup or on first render from the longest possible state string (including max timeout digits and prefix).
