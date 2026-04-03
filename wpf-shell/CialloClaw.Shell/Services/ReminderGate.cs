namespace CialloClaw.Shell.Services;

public sealed class ReminderGate
{
    private DateTime _lastReminderAt = DateTime.MinValue;
    private DateTime _bucketStart = DateTime.MinValue;
    private int _bucketCount;

    public bool CanEmit(int minIntervalSeconds, int maxPerHour)
    {
        var now = DateTime.Now;

        if ((now - _lastReminderAt).TotalSeconds < minIntervalSeconds)
        {
            return false;
        }

        if (_bucketStart == DateTime.MinValue || (now - _bucketStart).TotalHours >= 1)
        {
            _bucketStart = now;
            _bucketCount = 0;
        }

        if (_bucketCount >= maxPerHour)
        {
            return false;
        }

        _lastReminderAt = now;
        _bucketCount++;
        return true;
    }
}
