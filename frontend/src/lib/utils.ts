import { clsx, type ClassValue } from "clsx"
import { twMerge } from "tailwind-merge"

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

export function formatPackets(packets: number): string {
  if (packets >= 1_000_000) return `${(packets / 1_000_000).toFixed(1)}M`
  if (packets >= 1_000) return `${(packets / 1_000).toFixed(1)}K`
  return packets.toLocaleString()
}

export function formatDuration(firstSeen: number, lastSeen: number): string {
  const ms = lastSeen - firstSeen
  if (ms < 0) return '0s'
  const seconds = Math.floor(ms / 1000)
  if (seconds < 60) return `${seconds}s`
  if (seconds < 3600) return `${Math.floor(seconds / 60)}m ${seconds % 60}s`
  return `${Math.floor(seconds / 3600)}h ${Math.floor((seconds % 3600) / 60)}m`
}

export function countryToFlag(code: string): string {
  if (!code || code.length !== 2) return '';
  const a = code.toUpperCase().charCodeAt(0) - 65 + 0x1F1E6;
  const b = code.toUpperCase().charCodeAt(1) - 65 + 0x1F1E6;
  return String.fromCodePoint(a, b);
}
