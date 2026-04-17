interface apiResponseBase {
  success: boolean;
  message: string;
  metadata: Record<string, unknown>;
}

export interface APIResponse<T> extends apiResponseBase {
  data: T;
}

export interface PaginatedAPIResponse<T> extends apiResponseBase {
  data: T[];
}

// eslint-disable-next-line @typescript-eslint/no-empty-object-type
export interface APIErrorResponse extends apiResponseBase {
}
