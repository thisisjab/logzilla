export type LogLevel = 0 | 1 | 2 | 3 | 4 | 5;

export interface LogRecord {
  id: string;
  source: string;
  message: string;
  level: LogLevel;
  timestamp: string;
  metadata: object | null;
}
