# Job Research

Job Research collects publicly available job postings for an operator to review and apply to independently.

## Language

**Public-page source**:
A job board searched from its first publicly accessible results page, subject to robots.txt permission.
_Avoid_: crawler, protected source

**Browser-rendered retry**:
An explicitly operator-authorized retry that renders an accessible public results page in a normal browser after standard fetching fails.
_Avoid_: stealth mode, bypass

**Access-control response**:
A response that asks for login, CAPTCHA, verification, or otherwise denies automated access. It ends research for that provider.
_Avoid_: retryable result
