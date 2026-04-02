"use client";

import {
  Check,
  MessageSquare,
  Image as ImageIcon,
  Shield,
  RotateCcw,
  ArrowRight,
} from "lucide-react";

interface SuccessScreenProps {
  status?: string;
  notificationMethod?: string;
  attachedImage?: string;
}

export default function SuccessScreen({
  status = "Notification delivered successfully",
  notificationMethod = "WhatsApp",
  attachedImage = "https://hebbkx1anhila5yf.public.blob.vercel-storage.com/screen-kbPIwLfcDzANXVwFiJe3MD7QQ7r6PU.png",
}: SuccessScreenProps) {
  return (
    <div className="min-h-dvh bg-gray-50 flex flex-col">
      {/* Header */}
      <header className="flex items-center justify-between px-4 py-3 md:px-6 lg:px-8">
        <span className="font-bold text-xl">TagMe</span>
        <div className="w-10 h-10 rounded-full bg-yellow-400 border-2 border-yellow-500 overflow-hidden">
          <div className="w-full h-full bg-amber-200 flex items-center justify-center text-xs">
            <span className="text-amber-800">U</span>
          </div>
        </div>
      </header>

      {/* Main Content */}
      <main className="flex-1 px-4 pb-6 md:px-6 lg:px-8">
        <div className="max-w-4xl mx-auto w-full">
          {/* Success Hero */}
          <div className="relative flex flex-col items-center pt-4 pb-6 md:pt-8 md:pb-8">
            {/* Yellow gradient background */}
            <div className="absolute top-0 left-1/2 -translate-x-1/2 w-64 md:w-96 h-32 md:h-48 bg-gradient-to-b from-yellow-100 to-transparent rounded-full blur-2xl" />

            {/* Success Icon */}
            <div className="relative w-28 h-28 md:w-36 md:h-36 bg-yellow-400 rounded-full flex items-center justify-center mb-6 shadow-lg">
              <div className="w-16 h-16 md:w-20 md:h-20 bg-yellow-500 rounded-full flex items-center justify-center">
                <Check
                  className="w-10 h-10 md:w-12 md:h-12 text-white"
                  strokeWidth={3}
                />
              </div>
            </div>

            <h1 className="text-3xl md:text-4xl font-bold text-gray-900 mb-2">
              Message Sent!
            </h1>
            <p className="text-gray-500 md:text-lg">{status}</p>

            {/* Status Badge */}
            <div className="mt-4 bg-gray-200 rounded-full px-4 py-2 flex items-center gap-2">
              <MessageSquare className="w-4 h-4 text-green-600" />
              <span className="text-sm font-medium text-gray-700">
                Notified via {notificationMethod}
              </span>
            </div>
          </div>

          {/* Two Column Layout for larger screens */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-5 mt-4">
            {/* Attached Media */}
            <section className="bg-white rounded-2xl p-4 md:p-5 shadow-sm">
              <div className="flex items-center justify-between mb-3">
                <span className="text-xs font-medium text-gray-400 uppercase tracking-wider">
                  Attached Media
                </span>
                <ImageIcon className="w-5 h-5 text-gray-300" />
              </div>
              <div className="rounded-xl overflow-hidden bg-gray-900 aspect-[4/3]">
                <img
                  src={attachedImage}
                  alt="Attached media"
                  className="w-full h-full object-cover"
                />
              </div>
            </section>

            {/* Right column content */}
            <div className="space-y-5">
              {/* Privacy Guard */}
              <section className="bg-lime-200 rounded-2xl p-5 md:p-6 relative overflow-hidden">
                <div className="absolute bottom-0 right-0 w-24 h-24 opacity-20">
                  <Shield className="w-full h-full text-lime-500" />
                </div>
                <div className="flex items-center gap-2 mb-2">
                  <Shield className="w-5 h-5 text-green-700" />
                  <h3 className="font-semibold text-gray-900">Privacy Guard</h3>
                </div>
                <p className="text-sm md:text-base text-gray-700 leading-relaxed relative z-10">
                  Your contact info remains private. The owner only sees a
                  temporary TagMe alias.
                </p>
              </section>

              {/* Follow-up */}
              <section className="bg-gray-100 rounded-2xl p-4 flex items-center justify-between gap-4">
                <div className="flex items-center gap-3 min-w-0">
                  <div className="w-10 h-10 bg-white rounded-full flex items-center justify-center flex-shrink-0">
                    <RotateCcw className="w-5 h-5 text-gray-400" />
                  </div>
                  <div className="min-w-0">
                    <p className="font-medium text-gray-900 text-sm">
                      Need to add more?
                    </p>
                    <p className="text-xs text-gray-500 truncate">
                      Send another update to the owner
                    </p>
                  </div>
                </div>
                <button className="px-4 py-2 bg-white rounded-xl text-sm font-medium text-gray-700 shadow-sm hover:bg-gray-50 transition-colors flex-shrink-0">
                  Follow up
                </button>
              </section>
            </div>
          </div>

          {/* CTA Button */}
          <button className="w-full md:max-w-md md:mx-auto bg-yellow-400 hover:bg-yellow-500 transition-colors text-gray-900 font-semibold rounded-2xl py-4 flex items-center justify-center gap-2 mt-6 active:scale-[0.98] touch-manipulation">
            <span>I&apos;m done</span>
            <ArrowRight className="w-5 h-5" />
          </button>
        </div>
      </main>
    </div>
  );
}
