import test from "node:test";
import assert from "node:assert/strict";

import { ApiError } from "./api";
import {
  getConversationStatusErrorMessage,
  getConversationViewModel,
  minutesSince,
} from "./conversation-status";

const FIXED_NOW = new Date("2026-04-06T10:30:00+07:00").getTime();

test("maps recent pending status to waiting without reminder", () => {
  const viewModel = getConversationViewModel(
    {
      conversation_id: "conv-1",
      status: "PENDING",
      created_at: "2026-04-06 10:28:00",
    },
    FIXED_NOW,
  );

  assert.equal(viewModel.state, "waiting");
  assert.equal(viewModel.reminderState, "hidden");
  assert.equal(viewModel.showFallback, false);
});

test("defers reminder when delivered status passes reminder threshold", () => {
  const viewModel = getConversationViewModel(
    {
      conversation_id: "conv-2",
      status: "DELIVERED",
      created_at: "2026-04-06 10:24:00",
    },
    FIXED_NOW,
  );

  assert.equal(viewModel.state, "waiting");
  assert.equal(viewModel.reminderState, "deferred");
  assert.equal(viewModel.reminderLabel, "Reminder unavailable");
  assert.match(viewModel.reminderHint, /future update/i);
});

test("shows fallback state after long no-response window", () => {
  const viewModel = getConversationViewModel(
    {
      conversation_id: "conv-3",
      status: "PENDING",
      created_at: "2026-04-06 10:10:00",
    },
    FIXED_NOW,
  );

  assert.equal(viewModel.state, "no_response");
  assert.equal(viewModel.showFallback, true);
  assert.equal(viewModel.reminderState, "deferred");
});

test("maps owner response states correctly", () => {
  const opened = getConversationViewModel(
    {
      conversation_id: "conv-4",
      status: "OPENED",
      created_at: "2026-04-06 10:20:00",
    },
    FIXED_NOW,
  );

  const onTheWay = getConversationViewModel(
    {
      conversation_id: "conv-5",
      status: "ON_THE_WAY",
      created_at: "2026-04-06 10:20:00",
    },
    FIXED_NOW,
  );

  assert.equal(opened.state, "owner_responded");
  assert.match(opened.title, /seen your message/i);
  assert.equal(onTheWay.state, "owner_responded");
  assert.match(onTheWay.title, /on the way/i);
});

test("maps resolved status to terminal resolved state", () => {
  const viewModel = getConversationViewModel(
    {
      conversation_id: "conv-6",
      status: "RESOLVED",
      created_at: "2026-04-06 10:18:00",
    },
    FIXED_NOW,
  );

  assert.equal(viewModel.state, "resolved");
  assert.equal(viewModel.reminderState, "hidden");
});

test("returns safe error messages for not found and server failures", () => {
  const notFound = new ApiError("conversation not found", 404);
  const unavailable = new ApiError("server exploded", 503);
  const generic = new Error("network timeout");

  assert.equal(
    getConversationStatusErrorMessage(notFound),
    "Conversation not found. Scan the QR code again.",
  );
  assert.equal(
    getConversationStatusErrorMessage(unavailable),
    "The status service is temporarily unavailable.",
  );
  assert.equal(
    getConversationStatusErrorMessage(generic),
    "network timeout",
  );
});

test("minutesSince returns null for invalid timestamps", () => {
  assert.equal(minutesSince("not-a-date", FIXED_NOW), null);
});
