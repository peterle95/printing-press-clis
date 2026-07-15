# Browser login

Run `hevy login` to open a headed Chromium Playwright context under the local
application data directory. Enter all credentials, CAPTCHA, and second-factor
challenges yourself. Passwords, cookies, and tokens are never printed or stored
by the CLI outside the browser profile. Remove local session state with
`hevy logout --yes`.

Install Chromium for Playwright when prompted by the Playwright error:
`go run github.com/playwright-community/playwright-go/cmd/playwright install chromium`.

