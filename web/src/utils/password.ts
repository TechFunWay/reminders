export const MIN_PASSWORD_LENGTH = 6
export const MAX_PASSWORD_BYTES = 72

export function passwordValidationError(password: string): string {
  if (Array.from(password).length < MIN_PASSWORD_LENGTH) {
    return `密码长度不能少于${MIN_PASSWORD_LENGTH}位`
  }
  if (new TextEncoder().encode(password).length > MAX_PASSWORD_BYTES) {
    return `密码长度不能超过${MAX_PASSWORD_BYTES}字节`
  }
  return ''
}
