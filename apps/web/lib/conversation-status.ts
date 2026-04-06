import {
  isApiError,
  type ConversationStatusResponse,
} from "@/lib/api";

const WAITING_THRESHOLD_MINUTES = 10;
const REMINDER_THRESHOLD_MINUTES = 3;
const FALLBACK_THRESHOLD_MINUTES = 15;

export type ScannerState =
  | "loading"
  | "waiting"
  | "no_response"
  | "owner_responded"
  | "resolved"
  | "error";

export type ReminderState = "hidden" | "deferred";

export interface ConversationStatusViewModel {
  state: ScannerState;
  eyebrow: string;
  title: string;
  body: string;
  toneClass: string;
  iconName: "clock" | "warning" | "success";
  reminderState: ReminderState;
  reminderLabel: string;
  reminderHint: string;
  showFallback: boolean;
  statusLabel: string;
}

export function parseJakartaTimestamp(value: string) {
  const parsed = new Date(`${value.replace(" ", "T")}+07:00`);

  if (Number.isNaN(parsed.getTime())) {
    return null;
  }

  return parsed;
}

export function minutesSince(value: string, now = Date.now()) {
  const parsed = parseJakartaTimestamp(value);

  if (!parsed) {
    return null;
  }

  return Math.max(0, Math.floor((now - parsed.getTime()) / 60000));
}

export function formatSentAt(value: string) {
  const parsed = parseJakartaTimestamp(value);

  if (!parsed) {
    return "just now";
  }

  return new Intl.DateTimeFormat("en-US", {
    dateStyle: "medium",
    timeStyle: "short",
  }).format(parsed);
}

export function formatAge(minutes: number | null) {
  if (minutes === null) {
    return "recently";
  }

  if (minutes < 1) {
    return "just now";
  }

  if (minutes === 1) {
    return "1 minute ago";
  }

  return `${minutes} minutes ago`;
}

function getReminderState(ageMinutes: number | null) {
  if (ageMinutes === null || ageMinutes < REMINDER_THRESHOLD_MINUTES) {
    return {
      reminderState: "hidden" as const,
      reminderLabel: "Reminder unavailable",
      reminderHint: "A reminder is not available yet.",
    };
  }

  return {
    reminderState: "deferred" as const,
    reminderLabel: "Reminder unavailable",
    reminderHint: "Reminder will be available in a future update.",
  };
}

export function getConversationViewModel(
  response: ConversationStatusResponse | null,
  now = Date.now(),
): ConversationStatusViewModel {
  if (!response) {
    return {
      state: "loading",
      eyebrow: "Message sent",
      title: "Checking conversation status",
      body: "We are loading the live status for your anonymous message.",
      toneClass: "border-amber-200 bg-amber-50 text-amber-900",
      iconName: "clock",
      reminderState: "hidden",
      reminderLabel: "Reminder unavailable",
      reminderHint: "Status needs to load first.",
      showFallback: false,
      statusLabel: "Loading",
    };
  }

  const ageMinutes = minutesSince(response.created_at, now);
  const isRecent =
    ageMinutes !== null && ageMinutes < WAITING_THRESHOLD_MINUTES;
  const isFallbackWindow =
    ageMinutes !== null && ageMinutes >= FALLBACK_THRESHOLD_MINUTES;
  const reminder = getReminderState(ageMinutes);

  if (response.status === "RESOLVED") {
    return {
      state: "resolved",
      eyebrow: "Resolved",
      title: "The conversation is resolved",
      body: "The owner marked this conversation as resolved.",
      toneClass: "border-emerald-200 bg-emerald-50 text-emerald-900",
      iconName: "success",
      reminderState: "hidden",
      reminderLabel: "Reminder unavailable",
      reminderHint: "Resolved conversations do not allow reminders.",
      showFallback: false,
      statusLabel: response.status,
    };
  }

  if (response.status === "OPENED" || response.status === "ON_THE_WAY") {
    return {
      state: "owner_responded",
      eyebrow: "Owner responded",
      title:
        response.status === "ON_THE_WAY"
          ? "The owner is on the way"
          : "The owner has seen your message",
      body:
        response.status === "ON_THE_WAY"
          ? "The owner has acknowledged the alert and is heading over."
          : "The owner has opened the message and can act on it now.",
      toneClass: "border-cyan-200 bg-cyan-50 text-cyan-900",
      iconName: "success",
      reminderState: "hidden",
      reminderLabel: "Reminder unavailable",
      reminderHint: "No reminder is needed after the owner has responded.",
      showFallback: false,
      statusLabel: response.status,
    };
  }

  if (response.status === "PENDING" || response.status === "DELIVERED") {
    if (isFallbackWindow) {
      return {
        state: "no_response",
        eyebrow: "No response",
        title: "The owner still has not replied",
        body: "Fallback guidance is shown below if you need to move on.",
        toneClass: "border-orange-200 bg-orange-50 text-orange-900",
        iconName: "warning",
        ...reminder,
        showFallback: true,
        statusLabel: response.status,
      };
    }

    return {
      state: "waiting",
      eyebrow: "Waiting",
      title: "The owner has not responded yet",
      body: isRecent
        ? "Your message has been relayed. The owner may still be receiving it."
        : "The owner has not replied yet.",
      toneClass: "border-amber-200 bg-amber-50 text-amber-900",
      iconName: "clock",
      ...reminder,
      showFallback: false,
      statusLabel: response.status,
    };
  }

  return {
    state: "error",
    eyebrow: "Status unavailable",
    title: "We could not interpret the current conversation status",
    body: "Please refresh the page or scan the QR code again.",
    toneClass: "border-slate-200 bg-slate-50 text-slate-900",
    iconName: "warning",
    reminderState: "hidden",
    reminderLabel: "Reminder unavailable",
    reminderHint: "Reminder is unavailable for this status.",
    showFallback: false,
    statusLabel: response.status,
  };
}

export function getConversationStatusErrorMessage(error: unknown) {
  if (isApiError(error)) {
    if (error.status === 404) {
      return "Conversation not found. Scan the QR code again.";
    }

    if (error.status >= 500) {
      return "The status service is temporarily unavailable.";
    }

    if (error.message) {
      return error.message;
    }
  }

  if (error instanceof Error && error.message) {
    return error.message;
  }

  return "Unable to load the conversation status right now.";
}
