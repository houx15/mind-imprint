import { useEffect, useState } from "react";
import {
  ArrowRight,
  BookOpen,
  Compass,
  GraduationCap,
  Sprout,
} from "lucide-react";
import type { CourseSummary } from "@mind-imprint/contracts";
import type { MeUser } from "../api/auth";
import { apiFetch } from "../api/client";
import { listReadings } from "../api/readings";
import { listWritings } from "../api/writings";
import { listProjects } from "../api/projects";
import { apiErrorText } from "../api/errorText";
import { coursePath, navigate } from "../routing";
import { recentLearning, type RecentLearning } from "./recentLearning";
import together from "./assets/learning-together-v3.webp";
import curious from "./assets/curious-learner-v2.webp";
import makers from "./assets/project-makers-v2.webp";
import bookmark from "./assets/yinji-bookmark.webp";

export { bookmark };

export function LearningHome({ user }: { user: MeUser }) {
  const [recent, setRecent] = useState<RecentLearning[] | null>(null);
  const [courses, setCourses] = useState<CourseSummary[] | null>(null);
  const [errors, setErrors] = useState<string[]>([]);
  const [courseError, setCourseError] = useState("");
  const [retry, setRetry] = useState(0);
  useEffect(() => {
    let alive = true;
    setRecent(null);
    setCourses(null);
    setErrors([]);
    setCourseError("");
    void Promise.allSettled([
      listReadings(),
      listWritings(),
      listProjects(),
    ]).then(([r, w, p]) => {
      if (!alive) return;
      setRecent(
        recentLearning(
          r.status === "fulfilled" ? r.value : [],
          w.status === "fulfilled" ? w.value : [],
          p.status === "fulfilled" ? p.value : [],
        ),
      );
      setErrors(
        [r, w, p].flatMap((result, i) =>
          result.status === "rejected"
            ? [
                `${["阅读", "写作", "项目"][i]}加载失败：${apiErrorText(result.reason)}`,
              ]
            : [],
        ),
      );
    });
    // The existing endpoint enforces course audience; no client-side catalog.
    void apiFetch<{ courses: CourseSummary[] }>("/api/v1/courses")
      .then((r) => {
        if (alive) setCourses(r.courses.slice(0, 2));
      })
      .catch((e) => {
        if (alive) {
          setCourseError(`课程加载失败：${apiErrorText(e)}`);
          setCourses([]);
        }
      });
    return () => {
      alive = false;
    };
  }, [retry]);
  const first = recent?.[0];
  return (
    <div className="learning-home">
      <header className="learning-topline">
        <span>学习空间</span>
        <span className="learning-profile">
          {user.display_name?.slice(0, 1) || "我"}
        </span>
      </header>
      <div className="learning-columns">
        <div className="learning-content">
          <header className="learning-welcome">
            <div>
              <span className="learning-eyebrow">MIND IMPRINT</span>
              <h1>你好，{user.display_name || "同学"}</h1>
              <p>阅读、写作，探索你关心的问题。</p>
            </div>
            <img src={together} alt="" />
          </header>
          <section aria-labelledby="continue-title">
            <div className="learning-section-head">
              <h2 id="continue-title">继续学习</h2>
              <span>最近的学习任务</span>
            </div>
            {errors.length > 0 && (
              <div className="learning-error" role="alert">
                {errors.map((e) => (
                  <p key={e}>{e}</p>
                ))}
                <button onClick={() => setRetry((n) => n + 1)}>重新加载</button>
              </div>
            )}
            {recent === null ? (
              <p className="learning-loading" role="status">
                正在加载学习记录…
              </p>
            ) : first ? (
              <>
                <article className="learning-feature">
                  <div className="learning-feature-art">
                    <img
                      src={first.kind === "项目" ? makers : curious}
                      alt=""
                    />
                  </div>
                  <div className="learning-feature-copy">
                    <span className="learning-eyebrow">
                      {first.kind} · {first.status}
                    </span>
                    <h3>{first.title}</h3>
                    {first.detail && (
                      <p className="learning-detail">
                        当前步骤：{first.detail}
                      </p>
                    )}
                    <button
                      className="learning-primary"
                      onClick={() => navigate(first.path)}
                    >
                      继续{first.kind}
                      <ArrowRight size={17} />
                    </button>
                  </div>
                </article>
                {recent.length > 1 && (
                  <div className="learning-recent">
                    {recent.slice(1).map((item) => (
                      <button
                        key={item.key}
                        onClick={() => navigate(item.path)}
                      >
                        <span className="learning-kind">{item.kind}</span>
                        <span className="learning-recent-title">
                          {item.title}
                        </span>
                        <span className="learning-recent-status">
                          {item.status}
                        </span>
                        <ArrowRight size={16} />
                      </button>
                    ))}
                  </div>
                )}
              </>
            ) : errors.length === 0 ? (
              <div className="learning-empty">
                <img src={curious} alt="" />
                <div>
                  <h3>开始一次新的学习</h3>
                  <p>从感兴趣的问题出发，或带来一篇想读的文章。</p>
                  <button
                    className="learning-primary"
                    onClick={() => navigate("/explore")}
                  >
                    探索话题
                    <ArrowRight size={17} />
                  </button>
                </div>
              </div>
            ) : null}
          </section>
          <section aria-labelledby="course-title">
            <div className="learning-section-head">
              <h2 id="course-title">发现课程</h2>
              <button
                className="learning-link"
                onClick={() => navigate("/courses")}
              >
                全部课程
                <ArrowRight size={16} />
              </button>
            </div>
            {courseError && (
              <div className="learning-error" role="alert">
                {courseError}
                <button onClick={() => setRetry((n) => n + 1)}>重新加载</button>
              </div>
            )}
            {courses === null ? (
              <p className="learning-loading" role="status">
                正在加载课程…
              </p>
            ) : (
              <div className="learning-courses">
                {courses.map((course, i) => (
                  <button
                    key={course.slug}
                    className="learning-course"
                    onClick={() => navigate(coursePath(course.slug))}
                  >
                    <img
                      src={course.coverUrl || (i === 0 ? curious : together)}
                      alt=""
                    />
                    <span>
                      <strong>{course.title}</strong>
                      <small>{course.time_label || "思维课程"}</small>
                    </span>
                    <ArrowRight size={19} />
                  </button>
                ))}
              </div>
            )}
            {courses?.length === 0 && !courseError && (
              <p className="learning-loading">暂无可用课程。</p>
            )}
          </section>
          <section className="learning-shortcuts" aria-label="学习入口">
            <button onClick={() => navigate("/readings")}>
              <BookOpen size={20} />
              <span>开始阅读</span>
              <ArrowRight size={16} />
            </button>
            <button onClick={() => navigate("/projects")}>
              <Compass size={20} />
              <span>我的项目</span>
              <ArrowRight size={16} />
            </button>
            <button onClick={() => navigate("/courses")}>
              <GraduationCap size={20} />
              <span>学习课程</span>
              <ArrowRight size={16} />
            </button>
          </section>
        </div>
        <aside className="learning-companion">
          <h2>印记</h2>
          <p className="learning-role">AI 学习伙伴</p>
          <img
            className="learning-bookmark"
            src={bookmark}
            alt="印记：青绿色折纸书签形象"
          />
          <p>在阅读、写作和项目中，印记与你一起梳理问题、检查证据。</p>
          <div className="learning-companion-foot">
            <Sprout size={24} />
            <h3>我的兴趣树</h3>
            <p>查看从学习中记录下来的兴趣与问题。</p>
            <button className="learning-link" onClick={() => navigate("/tree")}>
              查看兴趣树
              <ArrowRight size={16} />
            </button>
          </div>
        </aside>
      </div>
    </div>
  );
}
