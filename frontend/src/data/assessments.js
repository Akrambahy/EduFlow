/**
 * Demo quizzes, assignments, projects, and grades.
 * MOCK DATA: Replace with backend API calls.
 */

/* ── Quizzes ─────────────────────────────────────────────── */

export const demoQuizzes = [
  {
    id: 'qz_001',
    courseId: 'crs_react',
    title: 'React Basics Quiz',
    description: 'Test your understanding of React fundamentals.',
    totalQuestions: 5,
    timeLimit: 15, // minutes
    dueDate: '2026-11-10',
    status: 'open',
    maxAttempts: 2,
  },
  {
    id: 'qz_002',
    courseId: 'crs_go',
    title: 'Go Concurrency Quiz',
    description: 'Goroutines, channels, and sync patterns.',
    totalQuestions: 8,
    timeLimit: 20,
    dueDate: '2026-11-12',
    status: 'open',
    maxAttempts: 1,
  },
];

export const demoQuizQuestions = {
  qz_001: [
    {
      id: 'qq_1',
      text: 'What hook is used to manage state in a functional component?',
      options: ['useEffect', 'useState', 'useContext', 'useReducer'],
      correctIndex: 1, // Backend authoritative — this is for demo display only
    },
    {
      id: 'qq_2',
      text: 'JSX stands for:',
      options: ['JavaScript XML', 'Java Syntax Extension', 'JSON XML', 'JavaScript Extension'],
      correctIndex: 0,
    },
    {
      id: 'qq_3',
      text: 'Which method is used to render a React component to the DOM?',
      options: ['React.render()', 'ReactDOM.createRoot()', 'React.mount()', 'ReactDOM.append()'],
      correctIndex: 1,
    },
    {
      id: 'qq_4',
      text: 'Props in React are:',
      options: ['Mutable', 'Read-only', 'Only for class components', 'Async'],
      correctIndex: 1,
    },
    {
      id: 'qq_5',
      text: 'useEffect runs:',
      options: ['Before render', 'After render', 'During render', 'Only once'],
      correctIndex: 1,
    },
  ],
};

export const demoQuizAttempts = [
  {
    id: 'qa_001',
    quizId: 'qz_001',
    studentId: 'usr_student_01',
    score: 4,
    total: 5,
    percentage: 80,
    submittedAt: '2026-11-03T09:30:00Z',
    answers: [1, 0, 1, 1, 1], // indexes chosen
  },
];

/* ── Assignments ─────────────────────────────────────────── */

export const demoAssignments = [
  {
    id: 'asg_001',
    courseId: 'crs_react',
    title: 'Build a Todo App',
    description: 'Create a todo list application using React with add, delete, and toggle functionality.',
    dueDate: '2026-11-15',
    maxGrade: 100,
    status: 'open',
  },
  {
    id: 'asg_002',
    courseId: 'crs_go',
    title: 'REST API Server',
    description: 'Implement a simple REST API server in Go with CRUD operations.',
    dueDate: '2026-11-20',
    maxGrade: 100,
    status: 'open',
  },
  {
    id: 'asg_003',
    courseId: 'crs_react',
    title: 'Component Library',
    description: 'Build a reusable component library with at least 5 components.',
    dueDate: '2026-10-30',
    maxGrade: 100,
    status: 'closed',
  },
];

export const demoSubmissions = [
  {
    id: 'sub_001',
    assignmentId: 'asg_003',
    studentId: 'usr_student_01',
    studentName: 'Marwan Ashour',
    submittedAt: '2026-10-29T14:00:00Z',
    fileName: 'component-library.zip',
    grade: 88,
    maxGrade: 100,
    feedback: 'Great work on the Button and Input components. Consider adding more accessibility attributes.',
    status: 'graded',
  },
];

/* ── Projects ────────────────────────────────────────────── */

export const demoProjects = [
  {
    id: 'prj_001',
    courseId: 'crs_react',
    trackId: 'trk_fullstack',
    title: 'Capstone: Full-Stack Dashboard',
    description: 'Build a complete dashboard application with authentication, data visualization, and real-time updates.',
    dueDate: '2026-12-15',
    status: 'in_progress',
    maxGrade: 100,
    teamSize: 3,
    requiresPresentation: true,
    githubUrl: null, // optional / deferred
  },
];

export const demoProjectSubmissions = [
  {
    id: 'psub_001',
    projectId: 'prj_001',
    studentId: 'usr_student_01',
    studentName: 'Marwan Ashour',
    status: 'in_progress',
    presentationDate: '2026-12-14',
    presentationTime: '14:00',
    grade: null,
    feedback: null,
  },
];

/* ── Grades & Standing ───────────────────────────────────── */

export const demoGrades = {
  usr_student_01: {
    courses: [
      { courseId: 'crs_react', courseName: 'React Fundamentals', quizAvg: 80, assignmentAvg: 88, projectGrade: null, attendance: 90, overall: 84 },
      { courseId: 'crs_go', courseName: 'Go Backend Basics', quizAvg: 75, assignmentAvg: null, projectGrade: null, attendance: 85, overall: 78 },
      { courseId: 'crs_db', courseName: 'Database Foundations', quizAvg: 90, assignmentAvg: 92, projectGrade: null, attendance: 95, overall: 92 },
      { courseId: 'crs_api', courseName: 'RESTful API Design', quizAvg: null, assignmentAvg: null, projectGrade: null, attendance: 100, overall: null },
    ],
    gpa: 84.7,
    standing: 'good', // Backend authoritative
  },
};

/* ── Notifications ───────────────────────────────────────── */

export const demoNotifications = [
  { id: 'ntf_001', type: 'assignment', title: 'New Assignment Posted', message: 'Build a Todo App has been posted in React Fundamentals.', read: false, createdAt: '2026-11-03T08:00:00Z', link: '/courses/crs_react/assignments/asg_001' },
  { id: 'ntf_002', type: 'grade', title: 'Assignment Graded', message: 'Your Component Library submission has been graded: 88/100.', read: false, createdAt: '2026-11-02T16:00:00Z', link: '/courses/crs_react/assignments/asg_003' },
  { id: 'ntf_003', type: 'session', title: 'Upcoming Session', message: 'React Components Workshop starts tomorrow at 10:00 AM.', read: true, createdAt: '2026-11-03T18:00:00Z', link: '/schedule' },
  { id: 'ntf_004', type: 'announcement', title: 'Welcome to EduFlow!', message: 'Your enrollment in the Full-Stack Development track has been confirmed.', read: true, createdAt: '2026-09-15T09:00:00Z', link: '/dashboard' },
];

/* ── Certificates ────────────────────────────────────────── */

export const demoCertificates = [
  {
    id: 'cert_001',
    studentId: 'usr_student_01',
    studentName: 'Marwan Ashour',
    courseId: 'crs_db',
    courseName: 'Database Foundations',
    issueDate: '2026-10-25',
    status: 'issued',
    verificationCode: 'EF-2026-DB-001',
    grade: 92,
  },
];

/* ── Announcements ───────────────────────────────────────── */

export const demoAnnouncements = [
  {
    id: 'ann_001',
    title: 'Mid-term Schedule Update',
    content: 'Mid-term assessments for all courses have been rescheduled to the week of November 18. Check your course pages for specific dates.',
    authorId: 'usr_admin_01',
    authorName: 'Sara Admin',
    authorRole: 'admin',
    createdAt: '2026-11-01T10:00:00Z',
    targetRoles: ['student', 'instructor', 'mentor'],
    courseId: null,
  },
  {
    id: 'ann_002',
    title: 'React Fundamentals: Project Guidelines',
    content: 'The final project guidelines have been posted. Teams of 3 are required. Please form your teams by November 8.',
    authorId: 'usr_instructor_01',
    authorName: 'Ahmed Khalil',
    authorRole: 'instructor',
    createdAt: '2026-11-02T14:00:00Z',
    targetRoles: ['student'],
    courseId: 'crs_react',
  },
];

