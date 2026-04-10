"use client";

import { useState } from "react";
import {
  ArrowLeft,
  Zap,
  CheckCircle,
  MessageSquare,
  ArrowRight,
  Send,
  Shield,
  Clock,
  Car,
  User,
} from "lucide-react";

interface FollowUpScreenProps {
  vehiclePlate?: string;
  sessionStatus?: "recognized" | "new";
}

export default function FollowUpScreen({
  vehiclePlate = "Unknown vehicle",
  sessionStatus = "recognized",
}: FollowUpScreenProps) {
  const [message, setMessage] = useState("");
  const [activeTab, setActiveTab] = useState("status");

  const handleSendFollowUp = () => {
    console.log("Sending follow-up:", message);
  };

  const handleStartNewSession = () => {
    console.log("Starting new session");
  };

  const tabs = [
    { id: "status", label: "STATUS", icon: Zap },
    { id: "history", label: "HISTORY", icon: Clock },
    { id: "vehicles", label: "VEHICLES", icon: Car },
    { id: "profile", label: "PROFILE", icon: User },
  ];

  return (
    <div className="min-h-screen bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="flex items-center justify-between px-4 py-4 bg-white">
        <button className="p-2 -ml-2">
          <ArrowLeft className="w-5 h-5 text-gray-800" />
        </button>
        <h1 className="text-lg font-semibold text-gray-900">Session Active</h1>
        <div className="w-10 h-10 bg-yellow-400 rounded-full flex items-center justify-center">
          <Zap className="w-5 h-5 text-gray-900" />
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 px-4 py-6 overflow-y-auto pb-24">
        {/* Session Badge */}
        {sessionStatus === "recognized" && (
          <div className="flex justify-center mb-6">
            <span className="inline-flex items-center gap-2 px-4 py-2 bg-lime-400 text-gray-900 text-xs font-semibold rounded-full">
              <CheckCircle className="w-4 h-4" />
              RECOGNIZED VISITOR
            </span>
          </div>
        )}

        {/* Heading */}
        <div className="text-center mb-6">
          <h2 className="text-3xl font-bold text-gray-900 mb-3">
            Sending a <span className="italic">follow-up?</span>
          </h2>
          <p className="text-gray-600 leading-relaxed">
            We recognize you. This will be added to your previous message to the
            owner of{" "}
            <span className="font-semibold text-gray-900">{vehiclePlate}</span>.
          </p>
        </div>

        {/* Quick Action Card */}
        <button className="w-full bg-white rounded-2xl p-4 shadow-sm flex items-center gap-4 mb-4">
          <div className="w-12 h-12 bg-gray-100 rounded-xl flex items-center justify-center">
            <MessageSquare className="w-5 h-5 text-gray-700" />
          </div>
          <div className="flex-1 text-left">
            <p className="font-semibold text-gray-900">Any updates?</p>
            <p className="text-sm text-gray-500">Send a quick nudge</p>
          </div>
          <ArrowRight className="w-5 h-5 text-gray-400" />
        </button>

        {/* Custom Message Textarea */}
        <div className="bg-white rounded-2xl p-4 shadow-sm mb-4">
          <textarea
            value={message}
            onChange={(e) => setMessage(e.target.value)}
            placeholder="Type your follow-up message here..."
            className="w-full h-32 resize-none text-gray-700 placeholder-gray-400 focus:outline-none"
          />
          <p className="text-xs text-gray-400 text-right font-medium">
            CUSTOM MESSAGE
          </p>
        </div>

        {/* Send Follow-up Button */}
        <button
          onClick={handleSendFollowUp}
          className="w-full bg-yellow-400 text-gray-900 font-semibold py-4 rounded-2xl flex items-center justify-center gap-2 mb-4"
        >
          Send Follow-up
          <Send className="w-5 h-5" />
        </button>

        {/* Start New Session Link */}
        <button
          onClick={handleStartNewSession}
          className="w-full text-center text-gray-700 font-semibold text-sm tracking-wide py-2 mb-8"
        >
          START A NEW SESSION
        </button>

        {/* Privacy Card */}
        <div className="bg-white rounded-2xl p-4 shadow-sm">
          <div className="flex items-start gap-3">
            <div className="w-10 h-10 bg-gray-100 rounded-full flex items-center justify-center flex-shrink-0">
              <Shield className="w-5 h-5 text-gray-600" />
            </div>
            <div>
              <h3 className="font-semibold text-gray-900 mb-1">
                Your privacy matters
              </h3>
              <p className="text-sm text-gray-500 leading-relaxed">
                We don&apos;t share your contact details. The owner only sees
                your messages via the secure TagMe relay.
              </p>
            </div>
          </div>
        </div>
      </main>

      {/* Bottom Navigation */}
      <nav className="fixed bottom-0 left-0 right-0 bg-white border-t border-gray-100 px-2 py-2">
        <div className="flex items-center justify-around max-w-md mx-auto">
          {tabs.map((tab) => {
            const Icon = tab.icon;
            const isActive = activeTab === tab.id;
            return (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                className={`flex flex-col items-center gap-1 px-4 py-2 rounded-xl transition-colors ${
                  isActive ? "bg-yellow-400" : ""
                }`}
              >
                <Icon
                  className={`w-5 h-5 ${isActive ? "text-gray-900" : "text-gray-400"}`}
                />
                <span
                  className={`text-xs font-medium ${isActive ? "text-gray-900" : "text-gray-400"}`}
                >
                  {tab.label}
                </span>
              </button>
            );
          })}
        </div>
      </nav>
    </div>
  );
}
