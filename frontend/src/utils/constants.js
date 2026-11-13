/* EduFlow – application-wide constants.
 * Change PRODUCT_NAME here to rebrand the entire UI. */

export const PRODUCT_NAME = 'EduFlow';

export const ROLES = {
  ADMIN: 'admin',
  INSTRUCTOR: 'instructor',
  MENTOR: 'mentor',
  TRACKER: 'tracker',
  STUDENT: 'student',
};

export const ROLE_LABELS = {
  [ROLES.ADMIN]: 'Administrator',
  [ROLES.INSTRUCTOR]: 'Instructor',
  [ROLES.MENTOR]: 'Mentor',
  [ROLES.TRACKER]: 'Tracker',
  [ROLES.STUDENT]: 'Student',
};

export const ATTENDANCE_STATUS = {
  PRESENT: 'present',
  ABSENT: 'absent',
  LATE: 'late',
  EXCUSED: 'excused',
};

export const SUBMISSION_STATUS = {
  PENDING: 'pending',
  SUBMITTED: 'submitted',
  GRADED: 'graded',
  RETURNED: 'returned',
};

export const STANDING = {
  GOOD: 'good',
  WARNING: 'warning',
  PROBATION: 'probation',
};

