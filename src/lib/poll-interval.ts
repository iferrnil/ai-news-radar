/** Shared fetch-frequency options for 数据源 table + 添加数据源 dialog. */
export const POLL_INTERVALS = [60, 120, 180, 300, 600, 1800, 3600, 10800, 21600] as const;
export type PollInterval = (typeof POLL_INTERVALS)[number];

export const pollLabel = (v: number) => (v < 3600 ? `${v / 60} 分钟` : `${v / 3600} 小时`);

/** 0 = server default (priority / Trendshift kind). */
export const POLL_DEFAULT = 0;
