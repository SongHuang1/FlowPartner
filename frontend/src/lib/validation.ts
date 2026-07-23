export function isPasswordStrong(pw: string): boolean {
  if (pw.length < 8) return false
  const hasUpper = /[A-Z]/.test(pw)
  const hasLower = /[a-z]/.test(pw)
  const hasDigit = /[0-9]/.test(pw)
  return hasUpper && hasLower && hasDigit
}
