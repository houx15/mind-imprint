import type { CSSProperties } from "react";
import type { MockCourse, CourseToneKind } from "./fixtures";
import { MOCK_COURSES } from "./fixtures";

const STAR = "M12 3l2.4 5 5.6.7-4 3.9 1 5.4L12 15.4 6.9 18l1-5.4-4-3.9L9.6 8z";

function toneStyle(kind: CourseToneKind): CSSProperties {
  const base: CSSProperties = {
    display: "inline-flex",
    alignItems: "center",
    gap: 5,
    fontSize: 12,
    fontWeight: 700,
    padding: "5px 11px",
    borderRadius: 999,
  };
  if (kind === "done") return { ...base, color: "#4C9A82", background: "#E7F3EE" };
  if (kind === "in_progress") return { ...base, color: "#2A3B7A", background: "#EDEFF9" };
  return { ...base, color: "#8A92A3", background: "#F1F2F5" };
}

function CourseCard({ course, onOpen }: { course: MockCourse; onOpen: () => void }) {
  return (
    <div
      onClick={onOpen}
      style={{
        display: "flex",
        flexDirection: "column",
        background: "#fff",
        border: "1px solid #EAECF2",
        borderRadius: 18,
        overflow: "hidden",
        cursor: "pointer",
        boxShadow: "0 1px 3px rgba(20,30,60,.04)",
      }}
    >
      {/* figure band */}
      <div
        style={{
          position: "relative",
          height: 120,
          background: "linear-gradient(135deg,#EDEFF9,#F3F0EC)",
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <span
          style={{
            position: "absolute",
            top: 14,
            left: 16,
            fontSize: 11,
            fontWeight: 700,
            color: "#2A3B7A",
            background: "rgba(255,255,255,.85)",
            padding: "4px 10px",
            borderRadius: 999,
          }}
        >
          {course.branch}
        </span>
        <div
          style={{
            width: 56,
            height: 56,
            borderRadius: 16,
            background: "#fff",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            boxShadow: "0 6px 16px rgba(42,59,122,.12)",
          }}
        >
          <svg width="30" height="30" viewBox="0 0 24 24" fill="none" stroke="#2A3B7A" strokeWidth="1.9" strokeLinecap="round" strokeLinejoin="round">
            <path d={STAR} />
          </svg>
        </div>
      </div>

      {/* body */}
      <div style={{ flex: 1, display: "flex", flexDirection: "column", padding: "18px 22px 20px" }}>
        <div style={{ fontSize: 18, fontWeight: 800, color: "#1C2333", lineHeight: 1.4 }}>{course.title}</div>
        <div style={{ fontSize: 13, color: "#6B7384", lineHeight: 1.66, marginTop: 8 }}>{course.blurb}</div>

        <div style={{ display: "flex", alignItems: "center", gap: 12, marginTop: 14, fontSize: 12, color: "#8A92A3", fontWeight: 600 }}>
          <span>{course.meta}</span>
          <span style={{ display: "inline-flex", alignItems: "center", gap: 5 }}>
            <svg width="13" height="13" viewBox="0 0 24 24" fill="none" stroke="#9AA1B0" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round">
              <circle cx="12" cy="12" r="9" />
              <path d="M12 7v5l3 2" />
            </svg>
            {course.time}
          </span>
        </div>

        {course.progressPct !== null && (
          <div style={{ display: "flex", alignItems: "center", gap: 10, marginTop: 14 }}>
            <div style={{ flex: 1, height: 6, background: "#EEF0F4", borderRadius: 999, overflow: "hidden" }}>
              <div style={{ width: `${course.progressPct}%`, height: "100%", background: "#2A3B7A" }} />
            </div>
            <span style={{ fontSize: 12, color: "#8A92A3", fontWeight: 700, flex: "none" }}>{course.progressPct}%</span>
          </div>
        )}

        <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between", gap: 10, marginTop: "auto", paddingTop: 16 }}>
          <span style={toneStyle(course.toneKind)}>
            {course.toneKind === "done" && (
              <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.6" strokeLinecap="round" strokeLinejoin="round">
                <path d="M20 6L9 17l-5-5" />
              </svg>
            )}
            {course.tone}
          </span>
          <button
            onClick={(e) => {
              e.stopPropagation();
              onOpen();
            }}
            style={{
              display: "inline-flex",
              alignItems: "center",
              gap: 6,
              background: "#2A3B7A",
              color: "#fff",
              border: "none",
              padding: "9px 15px",
              borderRadius: 10,
              fontSize: 13,
              fontWeight: 700,
              cursor: "pointer",
              fontFamily: "inherit",
            }}
          >
            {course.ctaLabel}
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.4" strokeLinecap="round" strokeLinejoin="round">
              <path d="M5 12h14M13 6l6 6-6 6" />
            </svg>
          </button>
        </div>
      </div>
    </div>
  );
}

export function CoursesView() {
  // Slice 1: course cards are inert; the course player arrives in Slice 4.
  const noop = () => {};
  return (
    <div style={{ height: "100%", minHeight: 0, overflowY: "auto" }}>
      <div style={{ maxWidth: 1000, margin: "0 auto", padding: "44px 40px 60px" }}>
        <div style={{ fontSize: 13, color: "#8A92A3", fontWeight: 600 }}>课程</div>
        <div style={{ fontSize: 28, fontWeight: 800, color: "#1C2333", marginTop: 6, letterSpacing: "-0.01em" }}>
          系统地学会一种思考方式
        </div>
        <div style={{ fontSize: 14, color: "#6B7384", marginTop: 8, lineHeight: 1.6, maxWidth: 560 }}>
          每一门课都是一段 AI 带着你走的学习旅程——有讲解，也有你亲自上手的挑战。学完，去「批判思维」工作台把它用在你自己的问题上。
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(2,1fr)", gap: 20, marginTop: 28 }}>
          {MOCK_COURSES.map((c) => (
            <CourseCard key={c.id} course={c} onOpen={noop} />
          ))}
        </div>
      </div>
    </div>
  );
}
