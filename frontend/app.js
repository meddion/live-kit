"use strict";

const API_BASE = "/api/v1";

const roomsList = document.getElementById("rooms-list");
const roomsStatus = document.getElementById("rooms-status");
const refreshBtn = document.getElementById("refresh-btn");
const createForm = document.getElementById("create-form");
const createStatus = document.getElementById("create-status");

const userBox = document.getElementById("user-box");
const userName = document.getElementById("user-name");
const logoutBtn = document.getElementById("logout-btn");

const loginCard = document.getElementById("login-card");
const loginForm = document.getElementById("login-form");
const loginStatus = document.getElementById("login-status");
const appContent = document.getElementById("app-content");

function showLoggedIn(identity) {
    userName.textContent = identity;
    userBox.hidden = false;
    loginCard.hidden = true;
    appContent.hidden = false;
}

function showLoggedOut() {
    userBox.hidden = true;
    appContent.hidden = true;
    loginCard.hidden = false;
}

async function fetchMe() {
    try {
        const res = await fetch(`${API_BASE}/me`);
        if (!res.ok) {
            showLoggedOut();
            return;
        }
        const data = await res.json();
        showLoggedIn(data.identity);
        fetchRooms();
    } catch (err) {
        showLoggedOut();
    }
}

async function readError(res) {
    const body = (await res.text()).trim();
    return body || `server responded with ${res.status}`;
}

async function fetchRooms() {
    roomsStatus.textContent = "Loading rooms...";
    roomsStatus.classList.remove("error");
    try {
        const res = await fetch(`${API_BASE}/rooms`);
        if (!res.ok) {
            throw new Error(await readError(res));
        }
        const data = await res.json();
        renderRooms(data.rooms || []);
    } catch (err) {
        roomsStatus.textContent = `Failed to load rooms: ${err.message}`;
        roomsStatus.classList.add("error");
    }
}

function formatRelativeTime(unixSeconds) {
    const diffSeconds = Math.floor(Date.now() / 1000) - unixSeconds;
    if (diffSeconds < 5) {
        return "just now";
    }

    const units = [
        ["day", 86400],
        ["hour", 3600],
        ["minute", 60],
        ["second", 1],
    ];
    for (const [label, seconds] of units) {
        const value = Math.floor(diffSeconds / seconds);
        if (value >= 1) {
            return `${value} ${label}${value === 1 ? "" : "s"} ago`;
        }
    }
    return "just now";
}

function renderRooms(rooms) {
    roomsList.innerHTML = "";
    if (rooms.length === 0) {
        roomsStatus.textContent = "No rooms available.";
        return;
    }
    roomsStatus.textContent = "";
    for (const room of rooms) {
        const li = document.createElement("li");

        const info = document.createElement("div");
        info.className = "room-info";

        const name = document.createElement("span");
        name.className = "room-name";
        name.textContent = room.name;

        const meta = document.createElement("span");
        meta.className = "room-meta";

        const parts = [];
        if (room.creation_time) {
            parts.push(`created ${formatRelativeTime(room.creation_time)}`);
        }

        const capacity = room.max_participants
            ? `${room.num_participants}/${room.max_participants}`
            : `${room.num_participants}`;
        parts.push(`👥 ${capacity}`);

        if (room.active_recording) {
            parts.push("🔴 recording");
        }

        meta.textContent = parts.join(" · ");

        info.append(name, meta);

        const joinBtn = document.createElement("button");
        joinBtn.type = "button";
        joinBtn.textContent = "Join";
        joinBtn.addEventListener("click", async () => {
            try {
                await joinRoom(room.name);
            } catch (err) {
                showCreateError(`Failed to join room: ${err.message}`);
            }
        });

        li.append(info, joinBtn);
        roomsList.append(li);
    }
}

async function joinRoom(room) {
    const res = await fetch(`${API_BASE}/rooms/${encodeURIComponent(room)}/join`, {
        method: "POST",
    });
    if (!res.ok) {
        throw new Error(await readError(res));
    }
    const data = await res.json();
    if (!data.join_url) {
        throw new Error("no join URL returned");
    }
    window.open(data.join_url, "_blank", "noopener");
}

createForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    clearCreateError();
    const room = document.getElementById("create-room").value.trim();
    try {
        await joinRoom(room);
        createForm.reset();
        fetchRooms();
    } catch (err) {
        showCreateError(`Failed to create room: ${err.message}`);
    }
});

function showCreateError(message) {
    createStatus.textContent = message;
    createStatus.classList.add("error");
    createStatus.hidden = false;
}

function clearCreateError() {
    createStatus.textContent = "";
    createStatus.classList.remove("error");
    createStatus.hidden = true;
}

refreshBtn.addEventListener("click", fetchRooms);

loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();
    loginStatus.hidden = true;
    loginStatus.classList.remove("error");
    const username = document.getElementById("login-username").value.trim();
    const password = document.getElementById("login-password").value;
    try {
        const res = await fetch(`${API_BASE}/login`, {
            method: "POST",
            headers: { "Content-Type": "application/json" },
            body: JSON.stringify({ username, password }),
        });
        if (!res.ok) {
            throw new Error(await readError(res));
        }
        const data = await res.json();
        loginForm.reset();
        showLoggedIn(data.identity);
        fetchRooms();
    } catch (err) {
        loginStatus.textContent = `Login failed: ${err.message}`;
        loginStatus.classList.add("error");
        loginStatus.hidden = false;
    }
});

logoutBtn.addEventListener("click", async () => {
    try {
        await fetch(`${API_BASE}/logout`, { method: "POST" });
    } catch (err) {
        // Ignore and reset the view regardless.
    }
    showLoggedOut();
});

fetchMe();
