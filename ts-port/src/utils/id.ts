/**
 * Short ID generation
 */

const ID_LENGTH = 6;
const ID_CHARS =
  "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789";

export function generateId(): string {
  let result = "";
  for (let i = 0; i < ID_LENGTH; i++) {
    result += ID_CHARS.charAt(Math.floor(Math.random() * ID_CHARS.length));
  }
  return result;
}
