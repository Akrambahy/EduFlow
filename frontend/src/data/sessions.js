/**
 * Demo sessions and attendance data.
 * MOCK DATA: Replace with GET /sessions and GET /attendance.
 */

export const demoSessions = [
  {
    id: 'ses_001',
    courseId: 'crs_react',
    title: 'React Components Workshop',
    date: '2026-11-04',
    startTime: '10:00',
    endTime: '12:00',
    location: 'Room 301',
    type: 'lecture',
    instructorId: 'usr_instructor_01',
    status: 'upcoming',
  },
  {
    id: 'ses_002',
    courseId: 'crs_react',
    title: 'State Management Lab',
    date: '2026-11-06',
    startTime: '10:00',
    endTime: '12:00',
    location: 'Lab 102',
    type: 'lab',
    instructorId: 'usr_instructor_01',
    status: 'upcoming',
  },
  {
    id: 'ses_003',
    courseId: 'crs_go',
    title: 'Concurrency Patterns',
    date: '2026-11-05',
    startTime: '14:00',
    endTime: '16:00',
    location: 'Room 205',
    type: 'lecture',
    instructorId: 'usr_instructor_01',
    status: 'upcoming',
  },
  {
    id: 'ses_past_001',
    courseId: 'crs_react',
    title: 'Introduction to React',
    date: '2026-10-28',
    startTime: '10:00',
    endTime: '12:00',
    location: 'Room 301',
    type: 'lecture',
    instructorId: 'usr_instructor_01',
    status: 'completed',
    qrCode: null,
  },
  {
    id: 'ses_past_002',
    courseId: 'crs_go',
    title: 'Go Basics',
    date: '2026-10-29',
    startTime: '14:00',
    endTime: '16:00',
    location: 'Room 205',
    type: 'lecture',
    instructorId: 'usr_instructor_01',
    status: 'completed',
  },
];

export const demoAttendance = [
  { id: 'att_001', sessionId: 'ses_past_001', studentId: 'usr_student_01', studentName: 'Marwan Ashour', status: 'present', checkedInAt: '2026-10-28T10:05:00Z', method: 'qr' },
  { id: 'att_002', sessionId: 'ses_past_002', studentId: 'usr_student_01', studentName: 'Marwan Ashour', status: 'late', checkedInAt: '2026-10-29T14:20:00Z', method: 'manual' },
];

/**
 * Student attendance summary – keyed by studentId.
 * MOCK DATA: Replace with GET /students/:id/attendance/summary
 */
export const demoAttendanceSummary = {
  usr_student_01: {
    totalSessions: 10,
    present: 7,
    late: 2,
    absent: 1,
    excused: 0,
    rate: 90,
  },
};

