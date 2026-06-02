"use client";

import { useEffect, useMemo, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import {
  AlertTriangle,
  Ban,
  Bell,
  Camera,
  ChevronRight,
  Lightbulb,
  MapPin,
  MessageCircle,
  MessageSquare,
  Shield,
  ScanLine,
  Settings,
} from "lucide-react";

import {
  getApiErrorMessage,
  getScanInfo,
  sendMessage,
  type ScannerMessageType,
} from "@/lib/api";

interface MessageOption {
  id: number;
  icon: typeof Ban;
  label: string;
  messageType: ScannerMessageType;
}

const messages: MessageOption[] = [
  {
    id: 1,
    icon: Ban,
    label: "You're blocking my car",
    messageType: "blocking",
  },
  {
    id: 2,
    icon: AlertTriangle,
    label: "Illegal parking detected",
    messageType: "illegal_parking",
  },
  {
    id: 3,
    icon: Lightbulb,
    label: "Lights are still on",
    messageType: "lights_on",
  },
  {
    id: 4,
    icon: MessageCircle,
    label: "Custom message...",
    messageType: "custom",
  },
];

export default function TagMePage() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const [selectedMessage, setSelectedMessage] = useState<number | null>(null);
  const [activeTab, setActiveTab] = useState("scan");
  const [customMessage, setCustomMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const qrToken = searchParams.get("token")?.trim() ?? "";
  const [plate, setPlate] = useState<string | null>(null);

  useEffect(() => {
    if (!qrToken) {
      setError("No QR token found. Please scan a valid QR code.");
      window.setTimeout(() => setError(null), 5000);
      return;
    }

    let cancelled = false;

    getScanInfo(qrToken)
      .then((data) => {
        if (cancelled) return;
        setPlate(data.plate ?? null);

        if (data.has_active && data.conversation_id) {
          router.push(`/conversations/${data.conversation_id}`);
        }
      })
      .catch((err) => {
        if (!cancelled) {
          console.error("[SCAN] getScanInfo error:", err);
          setError("QR code not found or inactive. Please scan again.");
          window.setTimeout(() => setError(null), 5000);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [qrToken, router]);

  const selectedMessageConfig = useMemo(
    () => messages.find((msg) => msg.id === selectedMessage) ?? null,
    [selectedMessage],
  );
  const isCustomMessage = selectedMessageConfig?.messageType === "custom";
  const canSubmit =
    !!qrToken &&
    !!selectedMessageConfig &&
    (!isCustomMessage || customMessage.trim().length > 0) &&
    !isSubmitting;

  async function handleSubmit() {
    if (!qrToken) {
      setError("Missing QR token. Please scan the QR code again.");
      return;
    }

    if (!selectedMessageConfig) {
      setError("Please select a message before sending.");
      return;
    }

    if (isCustomMessage && !customMessage.trim()) {
      setError("Please enter your custom message.");
      return;
    }

    setIsSubmitting(true);
    setError(null);

    try {
      const payload = {
        qr_token: qrToken,
        message_type: selectedMessageConfig.messageType,
        ...(isCustomMessage && customMessage.trim()
          ? { content: customMessage.trim() }
          : {}),
      };

      const data = await sendMessage(payload);
      const conversationId = data.conversation_id;

      if (!conversationId) {
        throw new Error(
          "Notification sent, but no conversation ID was returned.",
        );
      }

      router.push(`/conversations/${conversationId}`);
    } catch (err) {
      setError(
        getApiErrorMessage(
          err,
          "Something went wrong while sending the notification.",
        ),
      );
    } finally {
      setIsSubmitting(false);
    }
  }

  return (
    <div className="flex min-h-dvh flex-col bg-gray-50">
      {error && (
        <div className="fixed top-4 left-4 right-4 sm:left-auto sm:right-4 sm:w-96 z-50 animate-in fade-in slide-in-from-top-2 duration-300">
          <div className="rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-900 shadow-lg flex items-center gap-3">
            <AlertTriangle className="h-5 w-5 shrink-0" />
            <span className="flex-1">{error}</span>
            <button
              type="button"
              onClick={() => setError(null)}
              className="text-red-700 hover:text-red-900 transition"
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
      <header className="border-b border-gray-200 bg-white px-4 py-4 shadow-sm md:px-6 lg:hidden">
        <div className="mx-auto flex w-full max-w-4xl items-center justify-between">
          <div className="flex items-center gap-2">
            <span className="rounded-full bg-yellow-400 p-2 text-gray-900">
              <ScanLine className="h-5 w-5" />
            </span>
            <div>
              <p className="font-semibold text-lg">TagMe</p>
            </div>
          </div>
          <button
            type="button"
            className="rounded-full border border-gray-200 p-2 text-gray-500"
            aria-label="Settings"
          >
            <Settings className="h-5 w-5" />
          </button>
        </div>
      </header>

      <nav className="fixed top-0 right-0 left-0 z-10 hidden items-center justify-between border-b border-gray-200 bg-white px-8 py-3 lg:flex">
        <div className="flex items-center gap-2">
          <span className="rounded-full bg-yellow-400 p-2 text-gray-900">
            <ScanLine className="h-5 w-5" />
          </span>
          <div>
            <p className="text-lg font-semibold text-gray-900">TagMe</p>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {[
            { id: "scan", label: "Scan", icon: ScanLine },
            { id: "status", label: "Messages", icon: MessageSquare },
            { id: "privacy", label: "Privacy", icon: Shield },
          ].map((item) => {
            const Icon = item.icon;
            const isActive = activeTab === item.id;

            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setActiveTab(item.id)}
                className={`flex items-center gap-2 px-4 py-2 rounded-xl transition-colors ${
                  isActive
                    ? "bg-yellow-400 text-gray-900"
                    : "text-gray-500 hover:text-gray-700 hover:bg-gray-100"
                }`}
              >
                <Icon className="w-5 h-5" />
                <span className="text-sm font-medium">{item.label}</span>
              </button>
            );
          })}
        </div>
      </nav>

      <main className="flex-1 overflow-auto space-y-6 px-4 py-6 md:px-6 lg:px-8 lg:pt-24">
        <div className="mx-auto w-full max-w-4xl">
          <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
            <div className="space-y-6">
              <section className="bg-white rounded-3xl p-6 relative overflow-hidden shadow-sm ring-1 ring-gray-100 md:p-8">
                <div className="absolute top-0 right-0 w-32 h-32 bg-yellow-100 rounded-full -translate-y-1/2 translate-x-1/2 opacity-60" />
                <div className="mb-4 flex items-center gap-2 text-sm font-medium text-yellow-700">
                  <MapPin className="h-4 w-4" />
                  Vehicle notification request
                </div>
                <h1 className="mb-2 text-3xl font-semibold tracking-tight text-gray-900 md:text-4xl">
                  Notify the owner
                </h1>
                <p className="mb-6 text-sm leading-6 text-gray-500 md:text-base">
                  Send a quick anonymous message without exposing your contact
                  information.
                </p>
                <div className="mx-auto max-w-[240px] rounded-2xl border-2 border-gray-800 bg-white px-6 py-4">
                  <p className="text-center text-2xl font-bold tracking-widest md:text-3xl">
                    {plate || "Loading..."}
                  </p>
                </div>
                <div className="mt-4 flex items-center justify-center gap-1 text-yellow-600">
                  <MapPin className="h-4 w-4" />
                  <p className="text-xs font-medium uppercase tracking-wide">
                    Powered by secure relay messaging
                  </p>
                </div>
              </section>

              <section className="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-100 md:p-8">
                <h2 className="mb-4 text-lg font-semibold text-gray-900">
                  Select Message
                </h2>
                <div className="grid grid-cols-2 gap-3">
                  {messages.map((msg) => {
                    const Icon = msg.icon;
                    return (
                      <button
                        key={msg.id}
                        type="button"
                        onClick={() => {
                          setSelectedMessage(msg.id);
                          setError(null);
                        }}
                        className={`p-4 rounded-2xl text-left transition-all hover:shadow-md active:scale-[0.98] border ${
                          selectedMessage === msg.id
                            ? "border-yellow-400 bg-yellow-100"
                            : "border-gray-200 bg-white"
                        }`}
                      >
                        <div className="items-center gap-3">
                          <div className="rounded-full p-2 text-yellow-700">
                            <Icon className="h-5 w-5" />
                          </div>
                          <span className="text-sm font-medium text-gray-900 md:text-base">
                            {msg.label}
                          </span>
                        </div>
                      </button>
                    );
                  })}
                </div>
              </section>

              {isCustomMessage ? (
                <section className="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-100 md:p-8">
                  <label
                    htmlFor="custom-message"
                    className="mb-2 block text-sm font-medium text-gray-700"
                  >
                    Custom message
                  </label>
                  <textarea
                    id="custom-message"
                    value={customMessage}
                    onChange={(event) => setCustomMessage(event.target.value)}
                    placeholder="Add a short note for the vehicle owner"
                    className="min-h-32 w-full rounded-2xl border border-gray-200 bg-white px-4 py-3 text-sm text-gray-900 outline-none transition focus:border-yellow-400 focus:ring-2 focus:ring-yellow-100"
                  />
                </section>
              ) : null}

            </div>

            <div className="space-y-6">
              {/* Attach Photo */}
              <section>
                <div className="bg-yellow-50 border-2 border-yellow-400 border-dashed rounded-3xl p-8 md:p-10 flex flex-col items-center hover:bg-yellow-100 transition-colors cursor-not-allowed">
                  <div className="w-14 h-14 md:w-16 md:h-16 bg-yellow-400 rounded-full flex items-center justify-center mb-3">
                    <Camera className="w-6 h-6 md:w-7 md:h-7 text-white" />
                  </div>
                  <p className="font-semibold text-gray-800">Attach a Photo</p>
                  <p className="text-sm text-gray-500 text-center">
                    Coming soon.
                  </p>
                </div>
              </section>

              <section className="rounded-3xl bg-white p-6 shadow-sm ring-1 ring-gray-100 md:p-8">
                <div className="mb-4 rounded-2xl bg-green-50 p-4 text-sm text-green-800 ring-1 ring-green-100">
                  Your message will be securely relayed to the vehicle owner.
                  Your contact information stays private.
                </div>
                <h2 className="mb-4 text-lg font-semibold text-gray-900">
                  Send Notification
                </h2>
                <div className="space-y-3">
                  <button
                    type="button"
                    disabled
                    className="w-full cursor-not-allowed rounded-2xl bg-gray-200 px-4 py-4 text-gray-500"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-gray-400">
                          <MessageSquare className="h-5 w-5 text-white" />
                        </div>
                        <div className="text-left">
                          <span className="block font-medium text-gray-700">
                            Send via WhatsApp
                          </span>
                          <span className="text-xs text-gray-500">
                            Coming soon
                          </span>
                        </div>
                      </div>
                      <ChevronRight className="h-5 w-5 text-gray-400" />
                    </div>
                  </button>

                  <button
                    type="button"
                    onClick={handleSubmit}
                    disabled={!canSubmit}
                    className={`w-full rounded-2xl px-4 py-4 transition-colors active:scale-[0.99] ${
                      canSubmit
                        ? "bg-yellow-400 text-gray-900 hover:bg-yellow-500 cursor-pointer"
                        : "cursor-not-allowed bg-yellow-200 text-gray-500"
                    }`}
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-3">
                        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-yellow-500">
                          <Bell className="h-5 w-5 text-white" />
                        </div>
                        <div className="text-left">
                          <span className="block font-medium">
                            {isSubmitting ? "Sending..." : "Send via TagMe App"}
                          </span>
                          <span className="text-xs text-gray-700/70">
                            {qrToken
                              ? "Uses the live scanner messaging API"
                              : "Missing QR token in URL"}
                          </span>
                        </div>
                      </div>
                      <ChevronRight className="h-5 w-5 text-yellow-700" />
                    </div>
                  </button>
                </div>
              </section>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
