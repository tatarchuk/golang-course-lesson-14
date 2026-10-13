// Package auth holds the security primitives of the API: bcrypt password hashing, the
// HS256 login tokens and the verified user stored in a request context. It imports
// neither handler nor middleware, so both of them can import it.
package auth
