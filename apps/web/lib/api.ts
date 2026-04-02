export async function sendMessage(payload: any) {
  const res = await fetch("http://localhost:8080/messages", {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
    },
    body: JSON.stringify(payload),
  });

  if (!res.ok) {
    throw new Error("Failed to send message");
  }

  return res.json();
}