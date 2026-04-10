"use client";

import Link from "next/link";
import { useParams } from "next/navigation";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertCircle,
  ArrowLeft,
  ArrowRight,
  Bell,
  Check,
  CheckCircle2,
  Clock3,
  LinkIcon,
  RefreshCcw,
  ScanLine,
  Shield,
  TriangleAlert,
} from "lucide-react";

import {
  getConversationStatus,
  isApiError,
  sendReminder,
  type ConversationStatusResponse,
} from "@/lib/api";
import {
  formatAge,
  formatSentAt,
  getConversationStatusErrorMessage,
  getConversationViewModel,
  minutesSince,
} from "@/lib/conversation-status";

const POLL_INTERVAL_MS = 5000;

const ICONS = {
  clock: Clock3,
  success: CheckCircle2,
  warning: TriangleAlert,
};

export default function ConversationStatusPage() {
  const params = useParams<{ id?: string }>();
  const conversationId = typeof params?.id === "string" ? params.id.trim() : "";

  const [statusResponse, setStatusResponse] =
    useState<ConversationStatusResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [isInitialLoading, setIsInitialLoading] = useState(true);
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [lastCheckedAt, setLastCheckedAt] = useState<Date | null>(null);
  const [isCopied, setIsCopied] = useState(false);
  const [isReminderLoading, setIsReminderLoading] = useState(false);
  const [reminderNotification, setReminderNotification] = useState<{
    type: "success" | "error";
    message: string;
  } | null>(null);

  const refreshStatus = useCallback(
    async (options?: { silent?: boolean }) => {
      if (!conversationId) {
        setError("Conversation ID is missing.");
        setIsInitialLoading(false);
        return;
      }

      if (!options?.silent) {
        setIsRefreshing(true);
      }

      try {
        const response = await getConversationStatus(conversationId);
        setStatusResponse(response);
        setError(null);
        setLastCheckedAt(new Date());
      } catch (cause) {
        setError(getConversationStatusErrorMessage(cause));

        if (isApiError(cause) && cause.status === 404) {
          setStatusResponse(null);
        }
      } finally {
        setIsInitialLoading(false);
        if (!options?.silent) {
          setIsRefreshing(false);
        }
      }
    },
    [conversationId],
  );

  useEffect(() => {
    let cancelled = false;

    if (!conversationId) {
      setIsInitialLoading(false);
      setError("Conversation ID is missing.");
      setStatusResponse(null);
      return () => {
        cancelled = true;
      };
    }

    void refreshStatus();

    const intervalId = window.setInterval(() => {
      if (!cancelled) {
        void refreshStatus({ silent: true });
      }
    }, POLL_INTERVAL_MS);

    return () => {
      cancelled = true;
      window.clearInterval(intervalId);
    };
  }, [conversationId, refreshStatus]);

  const handleCopyLink = useCallback(async () => {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setIsCopied(true);
      window.setTimeout(() => {
        setIsCopied(false);
      }, 2000);
    } catch {
      setError("Unable to copy the conversation link right now.");
    }
  }, []);

  const handleSendReminder = useCallback(async () => {
    if (!conversationId) {
      setReminderNotification({
        type: "error",
        message: "Conversation ID is missing.",
      });
      return;
    }

    setIsReminderLoading(true);
    setReminderNotification(null);

    try {
      const response = await sendReminder(conversationId);

      if (!response.success) {
        const errorMessage =
          response.reason === "cooldown"
            ? "Please wait before sending another reminder."
            : response.reason === "limit_reached"
              ? "You have reached the reminder limit for this conversation."
              : response.reason === "already_sent"
                ? response.message || "Reminder already sent"
                : "Your daily limit has been reached. Please try again later.";

        setReminderNotification({
          type: "error",
          message: errorMessage,
        });
      } else {
        // Show success notification
        setReminderNotification({
          type: "success",
          message: "Reminder sent successfully",
        });
      }
      // Auto-dismiss after 4 seconds
      window.setTimeout(() => {
        setReminderNotification(null);
      }, 4000);
    } catch (cause) {
      setReminderNotification({
        type: "error",
        message: isApiError(cause)
          ? cause.message || "Failed to send reminder."
          : "Failed to send reminder.",
      });
      window.setTimeout(() => {
        setReminderNotification(null);
      }, 4000);
    } finally {
      setIsReminderLoading(false);
    }
  }, [conversationId]);

  const viewModel = useMemo(
    () => getConversationViewModel(statusResponse),
    [statusResponse],
  );

  const sentAt = statusResponse ? formatSentAt(statusResponse.created_at) : "";
  const ageLabel = statusResponse
    ? formatAge(minutesSince(statusResponse.created_at))
    : "";
  const canOpenFallback = viewModel.showFallback;
  const hasTerminalError = Boolean(error && !statusResponse);
  const isConversationUnavailable = !conversationId || hasTerminalError;
  const StatusIcon = ICONS[viewModel.iconName];

  return (
    <div className="min-h-dvh bg-[radial-gradient(circle_at_top,_rgba(250,204,21,0.24),_transparent_36%),linear-gradient(180deg,_#fffdf5_0%,_#fff8e7_52%,_#f8fafc_100%)] text-slate-900">
      {reminderNotification && (
        <div className="fixed top-4 left-4 right-4 sm:left-auto sm:right-4 sm:w-96 z-50 animate-in fade-in slide-in-from-top-2 duration-300">
          <div
            className={`rounded-2xl border px-4 py-3 text-sm shadow-lg flex items-center gap-3 ${
              reminderNotification.type === "success"
                ? "border-green-200 bg-green-50 text-green-900"
                : "border-red-200 bg-red-50 text-red-900"
            }`}
          >
            {reminderNotification.type === "success" ? (
              <CheckCircle2 className="h-5 w-5 shrink-0" />
            ) : (
              <AlertCircle className="h-5 w-5 shrink-0" />
            )}
            <span className="flex-1">{reminderNotification.message}</span>
            <button
              type="button"
              onClick={() => setReminderNotification(null)}
              className={`transition ${
                reminderNotification.type === "success"
                  ? "text-green-700 hover:text-green-900"
                  : "text-red-700 hover:text-red-900"
              }`}
            >
              <span className="sr-only">Dismiss</span>
              <svg className="h-4 w-4" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                  clipRule="evenodd"
                />
              </svg>
            </button>
          </div>
        </div>
      )}
      <header className="mx-auto flex w-full max-w-6xl items-center justify-between px-4 py-4 sm:px-6 lg:px-8">
        <div className="flex items-center gap-3">
          <Link
            href="/scan"
            className="inline-flex h-10 w-10 items-center justify-center rounded-full border border-white/70 bg-white/80 shadow-sm backdrop-blur transition hover:bg-white"
            aria-label="Back to scan"
          >
            <ArrowLeft className="h-4 w-4" />
          </Link>
          <div>
            <p className="text-xs font-semibold uppercase tracking-[0.25em] text-slate-500">
              Tag Me
            </p>
            <h1 className="text-lg font-semibold sm:text-xl">
              Conversation status
            </h1>
          </div>
        </div>

        <div className="hidden items-center gap-3 sm:flex">
          <span className="rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-medium text-amber-900">
            Anonymous relay
          </span>
          <button
            type="button"
            onClick={() => void handleCopyLink()}
            className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:border-slate-300 hover:text-slate-900 cursor-pointer"
          >
            {isCopied ? (
              <Check className="h-4 w-4" />
            ) : (
              <LinkIcon className="h-4 w-4" />
            )}
            {isCopied ? "Copied" : "Copy link"}
          </button>
          <Link
            href="/scan"
            className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:border-slate-300 hover:text-slate-900 cursor-pointer"
          >
            <ScanLine className="h-4 w-4" />
            New scan
          </Link>
        </div>
      </header>

      <main className="mx-auto w-full max-w-6xl px-4 pb-12 sm:px-6 lg:px-8">
        {isConversationUnavailable ? (
          <div className="mt-6 rounded-[2rem] border border-red-200 bg-red-50 p-6 text-red-900 shadow-sm">
            <div className="flex items-start gap-3">
              <TriangleAlert className="mt-0.5 h-5 w-5 shrink-0" />
              <div className="space-y-3">
                <h2 className="text-lg font-semibold">
                  Conversation unavailable
                </h2>
                <p className="text-sm leading-6">{error}</p>
                <div className="flex flex-wrap gap-3">
                  <button
                    type="button"
                    onClick={() => void refreshStatus()}
                    className="inline-flex items-center gap-2 rounded-2xl bg-red-950 px-4 py-2.5 text-sm font-semibold text-white transition hover:bg-red-900"
                  >
                    Retry
                  </button>
                  <Link
                    href="/scan"
                    className="inline-flex items-center gap-2 rounded-2xl border border-red-200 bg-white px-4 py-2.5 text-sm font-semibold text-red-900 transition hover:border-red-300"
                  >
                    Back to scan
                  </Link>
                </div>
              </div>
            </div>
          </div>
        ) : (
          <>
            <section className="grid gap-6 lg:grid-cols-[1.15fr_0.85fr]">
              <div className="rounded-[2rem] border border-white/80 bg-white/80 p-6 shadow-[0_18px_60px_rgba(15,23,42,0.08)] backdrop-blur sm:p-8">
                <div className="flex flex-col gap-6">
                  <div className="flex flex-wrap items-start justify-between gap-4">
                    <div className="space-y-3">
                      <div className="inline-flex items-center gap-2 rounded-full border border-amber-200 bg-amber-50 px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-amber-900">
                        <Bell className="h-3.5 w-3.5" />
                        Message sent
                      </div>
                      <div>
                        <h2 className="text-3xl font-semibold tracking-tight sm:text-4xl">
                          Your message is in the relay
                        </h2>
                        <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600 sm:text-base">
                          The owner will receive the alert without seeing your
                          contact details.
                        </p>
                      </div>
                    </div>

                    <button
                      type="button"
                      onClick={() => void refreshStatus()}
                      className="inline-flex items-center gap-2 rounded-full border border-slate-200 bg-white px-4 py-2 text-sm font-medium text-slate-700 shadow-sm transition hover:border-slate-300 hover:text-slate-900 cursor-pointer"
                    >
                      <RefreshCcw
                        className={`h-4 w-4 ${isRefreshing ? "animate-spin" : ""}`}
                      />
                      Refresh
                    </button>
                  </div>

                  <div className="grid gap-4 sm:grid-cols-2">
                    <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
                        Sent at
                      </p>
                      <p className="mt-2 text-sm text-slate-900">
                        {sentAt || "Waiting for server time"}
                      </p>
                    </div>

                    <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                      <p className="text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
                        Last checked
                      </p>
                      <p className="mt-2 text-sm text-slate-900">
                        {lastCheckedAt
                          ? new Intl.DateTimeFormat("en-US", {
                              timeStyle: "short",
                            }).format(lastCheckedAt)
                          : "Checking now"}
                      </p>
                    </div>
                  </div>

                  <div className="rounded-[1.75rem] border border-slate-200 bg-slate-50 p-5 sm:p-6">
                    <div className="flex items-start gap-4">
                      <div
                        className={`flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border ${viewModel.toneClass}`}
                      >
                        <StatusIcon className="h-6 w-6" />
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex flex-wrap items-center gap-2">
                          <span className="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-semibold uppercase tracking-[0.18em] text-slate-500">
                            {viewModel.eyebrow}
                          </span>
                          {statusResponse ? (
                            <span className="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600">
                              Status: {viewModel.statusLabel}
                            </span>
                          ) : null}
                          {ageLabel ? (
                            <span className="rounded-full border border-slate-200 bg-white px-3 py-1 text-xs font-medium text-slate-600">
                              {ageLabel}
                            </span>
                          ) : null}
                        </div>

                        <h3 className="mt-3 text-2xl font-semibold tracking-tight">
                          {viewModel.title}
                        </h3>
                        <p className="mt-3 max-w-2xl text-sm leading-6 text-slate-600 sm:text-base">
                          {viewModel.body}
                        </p>
                      </div>
                    </div>

                    {error ? (
                      <div className="mt-5 rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                        {error}
                      </div>
                    ) : null}
                  </div>

                  <div className="flex flex-wrap gap-3">
                    <Link
                      href="/scan"
                      className="inline-flex items-center justify-center gap-2 rounded-2xl bg-slate-900 px-5 py-3 text-sm font-semibold text-white shadow-sm transition hover:bg-slate-800 cursor-pointer"
                    >
                      Scan another QR
                      <ArrowRight className="h-4 w-4" />
                    </Link>

                    {statusResponse?.can_follow_up ? (
                      <button
                        type="button"
                        onClick={() => void handleSendReminder()}
                        disabled={isReminderLoading}
                        className={`inline-flex items-center justify-center gap-2 rounded-2xl px-5 py-3 text-sm font-semibold transition ${
                          isReminderLoading
                            ? "cursor-not-allowed bg-amber-100 text-amber-600"
                            : "border border-amber-200 bg-amber-50 text-amber-900 hover:bg-amber-100 cursor-pointer"
                        }`}
                      >
                        {isReminderLoading ? "Sending..." : "Send follow-up"}
                        <Bell className="h-4 w-4" />
                      </button>
                    ) : null}

                    {viewModel.reminderState === "deferred" ? (
                      <button
                        type="button"
                        disabled
                        className="cursor-not-allowed inline-flex items-center justify-center gap-2 rounded-2xl border border-slate-200 bg-slate-100 px-5 py-3 text-sm font-semibold text-slate-400"
                        title={viewModel.reminderHint}
                      >
                        {viewModel.reminderLabel}
                        <Bell className="h-4 w-4" />
                      </button>
                    ) : null}
                  </div>
                </div>
              </div>

              <aside className="space-y-6">
                <div className="rounded-[2rem] border border-white/80 bg-slate-950 p-6 text-white shadow-[0_18px_60px_rgba(15,23,42,0.16)] sm:p-7">
                  <div className="absolute pb-5 right-10 w-30 h-30 opacity-20">
                    <Shield className="w-full h-full text-amber-500" />
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="flex h-11 w-11 items-center justify-center rounded-2xl bg-white/10">
                      <Shield className="h-5 w-5 text-amber-300" />
                    </div>
                    <div>
                      <p className="text-xs font-semibold uppercase tracking-[0.2em] text-white/55">
                        Privacy guard
                      </p>
                      <h3 className="text-lg font-semibold">
                        Anonymous by design
                      </h3>
                    </div>
                  </div>

                  <p className="mt-4 text-sm leading-6 text-white/70">
                    Your contact information stays private. The owner only sees
                    the message relay and the current conversation state.
                  </p>

                  {/* <div className="mt-5 rounded-2xl border border-white/10 bg-white/5 p-4">
                    <p className="text-xs font-semibold uppercase tracking-[0.18em] text-white/55">
                      Live status
                    </p>
                    <p className="mt-2 text-sm text-white/80">
                      {statusResponse
                        ? `${statusResponse.status} is the latest update for this conversation.`
                        : "Waiting for the first status response."}
                    </p>
                  </div> */}
                </div>
              </aside>
            </section>

            {isInitialLoading && !statusResponse ? (
              <div className="mt-6 rounded-[2rem] border border-dashed border-slate-300 bg-white/80 p-6 text-sm text-slate-500 shadow-sm">
                Loading conversation status...
              </div>
            ) : null}

            {statusResponse && canOpenFallback ? (
              <div className="mt-6 rounded-[2rem] border border-slate-200 bg-white p-6 shadow-sm sm:p-7">
                <div className="flex flex-wrap items-center justify-between gap-3">
                  <div>
                    <p className="text-xs font-semibold uppercase tracking-[0.2em] text-slate-500">
                      Fallback guidance
                    </p>
                    <h2 className="mt-2 text-xl font-semibold">
                      The owner has not replied in time
                    </h2>
                  </div>
                </div>

                <div className="mt-5 grid gap-4 md:grid-cols-3">
                  <div className="rounded-2xl bg-slate-50 p-4">
                    <p className="text-sm font-semibold text-slate-900">
                      Try again later
                    </p>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      If the vehicle owner is unavailable, wait a bit and
                      refresh the status.
                    </p>
                  </div>
                  <div className="rounded-2xl bg-slate-50 p-4">
                    <p className="text-sm font-semibold text-slate-900">
                      Use the reminder flow
                    </p>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      The reminder CTA stays available while the conversation is
                      still actionable.
                    </p>
                  </div>
                  <div className="rounded-2xl bg-slate-50 p-4">
                    <p className="text-sm font-semibold text-slate-900">
                      Escalate locally
                    </p>
                    <p className="mt-2 text-sm leading-6 text-slate-600">
                      If needed, contact the nearby security team or on-site
                      staff.
                    </p>
                  </div>
                </div>
              </div>
            ) : null}
          </>
        )}
      </main>
    </div>
  );
}
