# Store JWT Access Tokens in localStorage for MVP

We decided to store the 24-hour JWT access token in browser `localStorage` and transmit it via the `Authorization: Bearer <token>` header, rather than implementing `httpOnly` cookies with CSRF mitigation or an access-and-refresh token rotation scheme. While storing tokens in JavaScript-accessible storage exposes them to theft in the event of an XSS vulnerability, this trade-off is accepted for the MVP to minimize infrastructure complexity, avoid CORS/cookie debugging overhead, and keep authentication purely stateless.
