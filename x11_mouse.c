/*
 * Bamboo - A Vietnamese Input method editor
 * Copyright (C) 2012 Le Quoc Tuan <mr.lequoctuan@gmail.com>
 * Copyright (C) 2018-2020 Luong Thanh Lam <ltlam93@gmail.com>
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

#include <X11/Xlib.h>
#include <stdlib.h>
#include <stdio.h>
#include <pthread.h>
#include <semaphore.h>
#include <sys/time.h>
#include <unistd.h>
#include <time.h>
#include "_cgo_export.h"
#define CAPTURE_MOUSE_MOVE_DELTA        50

static pthread_t th_mcap;
/*
 * sem_mcap is a counting semaphore used as a cross-thread signal.
 * Each sem_post() from mouse_capture_unlock() (triggered by a preedit update)
 * causes the capture thread to grab the pointer once.  Using a semaphore instead
 * of a mutex avoids the POSIX-undefined "unlock from a different thread" and
 * "same-thread double-lock" patterns that the old mutex code relied on.
 */
static sem_t sem_mcap;
static Display* dpy;
static volatile int mcap_running;

static void delay_ms(long msec) {
    struct timespec ts;
    ts.tv_sec  = msec / 1000;
    ts.tv_nsec = (msec % 1000) * 1000 * 1000;
    nanosleep(&ts, NULL);
}

/* returns 1 on success, 0 if grab failed or mcap_running became 0 */
static int grabPointer(Display *d, Window w) {
    int rc;
    while (mcap_running == 1) {
        rc = XGrabPointer(d, w, 0, ButtonPressMask | PointerMotionMask,
                          GrabModeAsync, GrabModeAsync, None, None, CurrentTime);
        switch (rc) {
            case GrabSuccess:
                return 1;
            case AlreadyGrabbed:
            case GrabFrozen:
                delay_ms(100);
                break;
            default:
                fprintf(stderr, "XGrabPointer failed (%d)\n", rc);
                return 0;
        }
    }
    return 0;
}

static void* thread_mouse_capture(void* data)
{
    XEvent event;
    int x_root_old = 0, y_root_old = 0;
    Window w, dummy_root, dummy_child;
    int dummy_x, dummy_y;
    unsigned int mask;

    dpy = XOpenDisplay(NULL);
    if (!dpy) {
        return NULL;
    }
    w = XDefaultRootWindow(dpy);
    XQueryPointer(dpy, w, &dummy_root, &dummy_child,
                  &x_root_old, &y_root_old, &dummy_x, &dummy_y, &mask);

    while (mcap_running == 1) {
        /*
         * Block here until updatePreedit signals that preedit text is active.
         * This ensures the pointer is only grabbed while there is something to
         * commit — eliminating the "idle grab" that prevented user clicks.
         */
        sem_wait(&sem_mcap);
        if (mcap_running == 0)
            break;

        if (!grabPointer(dpy, w))
            continue;

        /* Poll for the first pointer event while the grab is held. */
        while (mcap_running == 1) {
            if (XPending(dpy) > 0) {
                XNextEvent(dpy, &event);   /* consume — prevents stale re-detection */
                break;
            }
            delay_ms(50);
        }
        XUngrabPointer(dpy, CurrentTime);
        XFlush(dpy);

        if (mcap_running == 0)
            break;

        if (event.type == MotionNotify) {
            if ((abs(event.xmotion.x_root - x_root_old) >= CAPTURE_MOUSE_MOVE_DELTA) ||
                (abs(event.xmotion.y_root - y_root_old) >= CAPTURE_MOUSE_MOVE_DELTA)) {
                mouse_move_handler();
                x_root_old = event.xmotion.x_root;
                y_root_old = event.xmotion.y_root;
            }
            /*
             * Small move: do NOT re-grab immediately.  The old code called
             * pthread_mutex_unlock here to re-trigger the grab, which kept the
             * pointer grabbed continuously during slow mouse movements and caused
             * clicks to be swallowed indefinitely.  Now we simply fall through to
             * sem_wait so the grab only resumes on the next preedit update.
             * XRecord (x11_record.c) still detects clicks passively and commits
             * preedit without blocking click delivery.
             */
        } else {
            mouse_click_handler();
        }
        /* Loop back to sem_wait: grab is idle until next preedit update. */
    }

    mcap_running = 0;
    XCloseDisplay(dpy);
    return NULL;
}

void mouse_capture_init()
{
    setbuf(stdout, NULL);
    setbuf(stderr, NULL);
    if (mcap_running == 1) {
        return;
    }
    XInitThreads();
    mcap_running = 1;
    sem_init(&sem_mcap, 0, 0);  /* start blocked: grab only when preedit signals */
    pthread_create(&th_mcap, NULL, &thread_mouse_capture, NULL);
    pthread_detach(th_mcap);
}

void mouse_capture_exit()
{
    if (mcap_running == 0) {
        return;
    }
    mcap_running = 0;
    sem_post(&sem_mcap);   /* wake thread so it can observe mcap_running == 0 */
}

/* Called by updatePreedit when preedit text is non-empty. */
void mouse_capture_unlock()
{
    if (mcap_running == 0) {
        return;
    }
    sem_post(&sem_mcap);
}

void mouse_capture_start_or_unlock()
{
    if (mcap_running == 0) {
        mouse_capture_init();
    }
    sem_post(&sem_mcap);
}
