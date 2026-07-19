"use client";

const BASE_SALT = "forgec2-credential-v1";
const ITERATIONS = 600000;
const MARKER = "FC2ENC:";

function deriveSalt(username?: string): BufferSource {
  const seed = username ? `${username}:${BASE_SALT}` : BASE_SALT;
  return new TextEncoder().encode(seed).buffer;
}

export async function deriveKey(password: string, username?: string): Promise<CryptoKey> {
  const enc = new TextEncoder();
  const keyMaterial = await crypto.subtle.importKey(
    "raw", enc.encode(password), "PBKDF2", false, ["deriveKey"],
  );
  return crypto.subtle.deriveKey(
    { name: "PBKDF2", salt: deriveSalt(username), iterations: ITERATIONS, hash: "SHA-256" },
    keyMaterial,
    { name: "AES-GCM", length: 256 },
    false,
    ["decrypt", "encrypt"],
  );
}

export async function decryptLoot(encrypted: string, key: CryptoKey): Promise<string> {
  if (!encrypted.startsWith(MARKER)) {
    return encrypted;
  }
  const data = base64ToUint8(encrypted.slice(MARKER.length));
  const nonce = data.slice(0, 12);
  const ciphertext = data.slice(12);
  const decrypted = await crypto.subtle.decrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    ciphertext,
  );
  return new TextDecoder().decode(decrypted);
}

export async function encryptLoot(plaintext: string, key: CryptoKey): Promise<string> {
  const nonce = crypto.getRandomValues(new Uint8Array(12));
  const enc = new TextEncoder();
  const ciphertext = await crypto.subtle.encrypt(
    { name: "AES-GCM", iv: nonce },
    key,
    enc.encode(plaintext),
  );
  const combined = new Uint8Array(nonce.length + ciphertext.byteLength);
  combined.set(nonce, 0);
  combined.set(new Uint8Array(ciphertext), nonce.length);
  return MARKER + uint8ToBase64(combined);
}

export function generateKey(): string {
  const key = crypto.getRandomValues(new Uint8Array(32));
  return uint8ToBase64(key);
}

function base64ToUint8(base64: string): Uint8Array {
  const binary = atob(base64);
  const bytes = new Uint8Array(binary.length);
  for (let i = 0; i < binary.length; i++) {
    bytes[i] = binary.charCodeAt(i);
  }
  return bytes;
}

function uint8ToBase64(bytes: Uint8Array): string {
  let binary = "";
  for (let i = 0; i < bytes.length; i++) {
    binary += String.fromCharCode(bytes[i]);
  }
  return btoa(binary);
}
