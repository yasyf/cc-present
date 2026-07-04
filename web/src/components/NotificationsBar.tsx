// The notifications strip: the recent toasts only. cc-present surfaces connection
// status in the document header (DocHeader's ConnectionFrame), so this drops the
// library NotificationsBar's built-in connection pill — rendering both duplicates
// the LIVE indicator.

import type { StreamNotification } from '@cc-interact/react';

export interface NotificationsBarProps {
  notifications: StreamNotification[];
  onDismiss: (id: string) => void;
}

export function NotificationsBar({ notifications, onDismiss }: NotificationsBarProps) {
  const recent = notifications.slice(-5).reverse();

  return (
    <aside className="notifications">
      <div className="notif-list">
        {recent.map((n) => (
          <div key={n.id} className={`notif notif-${n.kind}`}>
            <span className="notif-msg">{n.text}</span>
            <button
              type="button"
              className="notif-x"
              aria-label="dismiss"
              onClick={() => onDismiss(n.id)}
            >
              ×
            </button>
          </div>
        ))}
      </div>
    </aside>
  );
}
