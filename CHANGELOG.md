# CHANGELOG (NHẬT KÝ THAY ĐỔI MÃ NGUỒN)

> File này ghi lại chi tiết các thay đổi mã nguồn, tính năng, refactor và fix bug trong suốt vòng đời dự án `zalo-cli`, theo đúng quy định tại [`AGENTS.md`](./AGENTS.md).

---

## 📜 Phase 0: Setup Workspace & Project Architecture

### [Phase 0 - Init] Thiết lập quy tắc Agent và Kế hoạch Tổng thể
* **Thời gian:** 2026-10-16 02:00
* **Scope:** `docs(workspace)`
* **Files Created/Modified:**
  - `AGENTS.md`: Định nghĩa quy tắc bắt buộc của Agent, tiêu chuẩn Git và định hướng Tech Stack.
  - `COMMAND_LOG.md`: Khởi tạo bảng theo dõi nhật ký lệnh thực thi.
  - `CHANGELOG.md`: Khởi tạo bảng theo dõi nhật ký thay đổi mã nguồn.
  - `plan.md`: Tài liệu kiến trúc và lộ trình thực thi theo micro-steps 15–30 phút.
* **Nguyên nhân kỹ thuật & Rationale:**
  - Thiết lập nền móng vững chắc, kỷ luật và có thể theo dõi tiến độ rõ ràng trước khi viết dòng code đầu tiên. Đảm bảo Bảo vừa thực hành vừa nắm trọn vẹn bản chất kiến trúc hệ thống Client – Daemon.
