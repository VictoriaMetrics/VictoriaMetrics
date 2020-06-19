`vmstorage` performs the following tasks:

- Accepts inserts from `vminsert` nodes and stores them to local storage.

- Performs select requests from `vmselect` nodes.  

**Usage of storageGroup**  
As data insertion increases, query efficiency will gradually decrease.Especially when the query time is long, it takes tens of seconds or even minutes to get the result. This will increase the system's slow query. At the same time, when the user needs to query in the past few days, weeks, and months, it needs to wait for a long time, causing a bad user experience and affecting the performance of the system.    
Use group storage to improve system performance. Each group is a separate storage space. Take the extreme values of N time points of each metrics information as new points and insert them into the group. This can increase query efficiency by a factor of N while consuming limited storage resources.  

-groupSwitch. Whether to use group storage.  
-storageGroups. Information of storageGroups.  
   usage: -storageGroups="step path queryRangeMin switch"   
   step: Data sampling point interval;   
   path:the path to storage data;  
   queryRangeMin:when queryTimeRange greater or equal to this queryRangeMin,this storage will be used;  
   switch:this group data is used or not;  
   for example:  
    -storageGroups="10 /data/vm/group1 24h true"  -storageGroups="50 /data/vm/group2 168h true"  
    this mean we create 2 groups storage.Group1 insert one metrics point every 10 point.Group2 insert one metrics point every 10 point.  
    When queryRange greater or equal to 168h,group2 will be used.


