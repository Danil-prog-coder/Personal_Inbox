export type Level = 'CRITICAL' | 'HIGH' | 'NORMAL' | 'LOW';
export type SourceKind = 'gmail' | 'telegram';
export type ConnState = 'off' | 'active' | 'reauth';
export type MsgStatus = 'PROCESSING' | 'DONE';
export type Theme = 'dark' | 'light';
export type Density = 'spacious' | 'compact';
export type Period = 'all' | 'today' | 'week' | 'month';
export type SummaryPeriod = '24h' | 'week' | 'month';

export interface User {
  id: number;
  email: string;
  criteria: string;
  theme: Theme;
  density: Density;
  created_at: string;
}

export interface Connection {
  kind: SourceKind;
  state: ConnState;
  account: string;
  last_sync_at: string | null;
}

export interface MessageBrief {
  id: number;
  sender_name: string;
  subject: string;
  level: Level;
}

export interface SourceCard {
  kind: SourceKind;
  state: ConnState;
  account: string;
  last_sync_at: string | null;
  total: number;
  unread: number;
  distribution: Record<Level, number>;
  urgent: MessageBrief | null;
}

export interface Message {
  id: number;
  source: SourceKind;
  external_id: string;
  sender_name: string;
  sender_addr: string;
  subject: string;
  body: string;
  received_at: string;
  is_read: boolean;
  status: MsgStatus;
  level: Level;
  level_override: Level | null;
  category: string;
  deadline_text: string;
  needs_reply: boolean;
  needs_action: boolean;
  summary: string;
  external_url: string;
  analyzed_at: string | null;
  analysis_failed: boolean;
}

export interface MessageList {
  items: Message[];
  total: number;
  unread: number;
}

export interface Summary {
  period: SummaryPeriod;
  total: number;
  distribution: Record<Level, number>;
  needs_reply: number;
  needs_action: number;
  top: MessageBrief[];
}

export interface Filters {
  level: 'all' | Level;
  status: 'all' | 'unread' | 'read' | 'done';
  reply: 'all' | 'yes' | 'no';
  action: 'all' | 'yes' | 'no';
  period: Period;
}

export const EMPTY_FILTERS: Filters = {
  level: 'all',
  status: 'all',
  reply: 'all',
  action: 'all',
  period: 'all',
};
