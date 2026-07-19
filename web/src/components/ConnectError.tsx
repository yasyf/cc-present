// ConnectError is the terminal fallback shown once the stream has neither caught
// up nor produced content after the connect timeout — a dead EventSource never
// retries, so BoardSkeleton would otherwise spin forever.
export function ConnectError() {
  return (
    <div className="connect-error">
      <div className="connect-error-title">Can't connect to this board</div>
      <div className="connect-error-sub">Check the URL, then reload.</div>
    </div>
  );
}
