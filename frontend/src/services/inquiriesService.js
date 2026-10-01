import { request } from "./api"
export const sendInquiry = (data) => request("/inquiries", { method: "POST", body: JSON.stringify(data) })
