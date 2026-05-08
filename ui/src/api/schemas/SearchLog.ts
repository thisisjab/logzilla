import type { LogRecord } from "../models/LogRecord";
import type { PaginatedAPIResponse } from "../models/Response";

export interface SearchLogRequest {
  query: string;
}

export interface SearchLogResponse extends PaginatedAPIResponse<LogRecord> {
  metadata: { cursor: string };
}
